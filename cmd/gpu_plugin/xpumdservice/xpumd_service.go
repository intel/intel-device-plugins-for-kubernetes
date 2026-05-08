// Copyright 2026 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xpumdservice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	xpumapi "github.com/intel/xpumanager/xpumd/exporter/api/deviceinfo/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

const (
	// DefaultSocketPath is the default xpumd socket path.
	DefaultSocketPath = "/run/xpumd/intelxpuinfo.sock"

	connectAttemptsMax     = 30
	connectAttemptInterval = 10 * time.Second
	arbitraryErrorDelay    = 5 * time.Second
	maxArbitraryErrors     = 5
)

// ErrNoHealthData is returned by GetDeviceHealth when no health data has been
// received yet for the requested device.
var ErrNoHealthData = errors.New("no health data available yet")

// DeviceHealthy contains per-device health information retrieved from xpumd.
type DeviceHealthy bool

// XpumdService is the interface for querying device health from xpumd.
type XpumdService interface {
	// Run starts the background streaming listener. It blocks until ctx is
	// canceled or a fatal error occurs.
	Run(ctx context.Context)
	// GetDeviceHealth returns the latest cached health for the given PCI BDF
	// address (e.g. "0000:03:00.0").  It returns an error when no data has
	// been received yet.
	GetDeviceHealth(bdfAddress string) (DeviceHealthy, error)
}

// NewXpumd creates a new XpumdService that connects to the given unix socket.
func NewXpumd(socketPath string) XpumdService {
	return &xpumd{
		socketPath: socketPath,
		healths:    make(map[string]DeviceHealthy),
	}
}

type xpumd struct {
	healths    map[string]DeviceHealthy
	socketPath string

	sync.RWMutex
}

func (x *xpumd) Run(ctx context.Context) {
	klog.V(3).Info("Starting xpumd listener. Connecting to: unix://", x.socketPath)

	conn, err := grpc.NewClient("unix://"+x.socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		klog.Error("xpumd-client: failed to create gRPC client, health monitoring will be disabled: ", err)
		return
	}

	defer conn.Close()

	client := xpumapi.NewDeviceInfoClient(conn)

	stream, err := x.waitForStream(ctx, client)
	if err != nil {
		if ctx.Err() == nil {
			klog.Error("xpumd-client: failed to connect to xpumd within expected time, giving up")
		}

		return
	}

	klog.V(5).Infof("xpumd-client: successfully connected at %s", x.socketPath)

	errCounter := 0

	for {
		msg, err := stream.Recv()
		if err != nil {
			if ctx.Err() != nil {
				klog.V(5).Info("xpumd-client: context canceled, stopping listener")
				return
			}

			if errors.Is(err, io.EOF) {
				klog.Errorf("xpumd-client: stream closed by remote (%v), reconnecting", err)

				stream, err = x.waitForStream(ctx, client)
				if err != nil {
					if ctx.Err() == nil {
						klog.Error("xpumd-client: failed to reconnect to xpumd, giving up")
					}

					return
				}

				continue
			}

			if errCounter < maxArbitraryErrors {
				klog.Errorf("xpumd-client: error receiving data (%v), retrying (%d/%d)", err, errCounter+1, maxArbitraryErrors)
				errCounter++

				time.Sleep(arbitraryErrorDelay)

				continue
			}

			klog.Errorf("xpumd-client: %d consecutive errors, giving up: %v", maxArbitraryErrors, err)

			return
		}

		errCounter = 0

		klog.V(5).Infof("xpumd-client: received health entries for %d devices", len(msg.GetDevices()))
		x.applyDeviceHealthUpdate(msg.GetDevices())

		if ctx.Err() != nil {
			klog.V(5).Info("xpumd-client: context canceled, stopping listener")
			return
		}
	}
}

func (x *xpumd) waitForStream(ctx context.Context, client xpumapi.DeviceInfoClient) (grpc.ServerStreamingClient[xpumapi.DeviceHealthResponse], error) {
	var (
		stream grpc.ServerStreamingClient[xpumapi.DeviceHealthResponse]
		err    error
	)

	for attempt := 0; attempt < connectAttemptsMax; attempt++ {
		klog.V(5).Infof("xpumd-client: WatchDeviceHealth attempt %d/%d", attempt+1, connectAttemptsMax)

		stream, err = client.WatchDeviceHealth(ctx, &xpumapi.WatchDeviceHealthRequest{})
		if err == nil || ctx.Err() != nil {
			break
		}

		klog.Errorf("xpumd-client: WatchDeviceHealth failed: %v", err)
		time.Sleep(connectAttemptInterval)
	}

	return stream, err
}

func (x *xpumd) applyDeviceHealthUpdate(devices []*xpumapi.DeviceHealth) {
	x.Lock()
	defer x.Unlock()

	for _, dev := range devices {
		info := dev.GetInfo()
		if info == nil || info.GetPci() == nil {
			continue
		}

		bdf := info.GetPci().GetBdf()
		healthy := true

		for _, hs := range dev.GetHealth() {
			if hs.GetSeverity() >= xpumapi.SeverityLevel_SEVERITY_LEVEL_WARNING {
				klog.V(5).Infof("xpumd-client: device %s health issue in domain %q: severity=%s reason=%q",
					bdf, hs.GetName(), hs.GetSeverity(), hs.GetReason())

				healthy = false
			}
		}

		x.healths[bdf] = DeviceHealthy(healthy)
	}
}

func (x *xpumd) GetDeviceHealth(bdfAddress string) (DeviceHealthy, error) {
	x.RLock()
	defer x.RUnlock()

	healthy, ok := x.healths[bdfAddress]
	if !ok {
		return true, fmt.Errorf("%w: %s", ErrNoHealthData, bdfAddress)
	}

	return healthy, nil
}
