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
	"flag"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	xpumapi "github.com/intel/xpumanager/xpumd/exporter/api/deviceinfo/v1alpha1"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

func init() {
	_ = flag.Set("v", "4") // enable debug output
}

// mockDeviceInfoServer implements xpumapi.DeviceInfoServer for testing.
type mockDeviceInfoServer struct {
	xpumapi.UnimplementedDeviceInfoServer
	responses []*xpumapi.DeviceHealthResponse
}

func (m *mockDeviceInfoServer) WatchDeviceHealth(_ *xpumapi.WatchDeviceHealthRequest, stream grpc.ServerStreamingServer[xpumapi.DeviceHealthResponse]) error {
	for _, resp := range m.responses {
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	// Keep stream open briefly so the client can receive all messages.
	time.Sleep(100 * time.Millisecond)
	return nil
}

func startMockServer(t *testing.T, responses []*xpumapi.DeviceHealthResponse) string {
	t.Helper()

	dir, err := os.MkdirTemp("", "testingxpumd*")
	if err != nil {
		t.Fatal("failed to create tmp directory:", err)
	}

	t.Cleanup(func() { os.RemoveAll(dir) })

	sockPath := filepath.Join(dir, "xpumd.sock")

	var lc net.ListenConfig

	lis, err := lc.Listen(context.Background(), "unix", sockPath)
	if err != nil {
		t.Fatal("failed to listen:", err)
	}

	s := grpc.NewServer()
	xpumapi.RegisterDeviceInfoServer(s, &mockDeviceInfoServer{responses: responses})

	go func() {
		if err := s.Serve(lis); err != nil {
			klog.V(4).Infof("mock server stopped: %v", err)
		}
	}()

	t.Cleanup(s.Stop)

	return sockPath
}

func createHealthResponse(bdf string, severity xpumapi.SeverityLevel, reason string) *xpumapi.DeviceHealthResponse {
	return &xpumapi.DeviceHealthResponse{
		Devices: []*xpumapi.DeviceHealth{
			{
				Info: &xpumapi.DeviceInformation{
					Pci: &xpumapi.PciInfo{Bdf: bdf},
				},
				Health: []*xpumapi.HealthStatus{
					{Name: "memory", Severity: severity, Reason: reason},
				},
			},
		},
	}
}

func retrieveDeviceHealth(bdf string, svc XpumdService) (DeviceHealthy, error) {
	// Wait until health data for the device is populated.
	deadline := time.Now().Add(5 * time.Second)

	var healthy DeviceHealthy

	var err error

	for time.Now().Before(deadline) {
		healthy, err = svc.GetDeviceHealth(bdf)
		if err == nil {
			break
		}

		time.Sleep(50 * time.Millisecond)
	}

	return healthy, err
}

func TestGetDeviceHealth_Healthy(t *testing.T) {
	const bdf = "0000:03:00.0"

	resp := createHealthResponse(bdf, xpumapi.SeverityLevel_SEVERITY_LEVEL_OK, "")

	sockPath := startMockServer(t, []*xpumapi.DeviceHealthResponse{resp})

	svc := NewXpumd(sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run in background; cancel after health data has been received.
	go func() {
		svc.Run(ctx)
	}()

	// Wait until health data for the device is populated.
	healthy, err := retrieveDeviceHealth(bdf, svc)

	if err != nil {
		t.Fatalf("GetDeviceHealth returned error: %v", err)
	}

	if !healthy {
		t.Errorf("expected device to be healthy, got unhealthy")
	}
}

func TestGetDeviceHealth_Unhealthy(t *testing.T) {
	const bdf = "0000:04:00.0"

	resp := createHealthResponse(bdf, xpumapi.SeverityLevel_SEVERITY_LEVEL_WARNING, "ecc")

	sockPath := startMockServer(t, []*xpumapi.DeviceHealthResponse{resp})

	svc := NewXpumd(sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		svc.Run(ctx)
	}()

	// Wait until health data for the device is populated.
	healthy, err := retrieveDeviceHealth(bdf, svc)

	if err != nil {
		t.Fatalf("GetDeviceHealth returned error: %v", err)
	}

	if healthy {
		t.Errorf("expected device to be unhealthy, got healthy")
	}
}

func TestGetDeviceHealth_NoDataYet(t *testing.T) {
	// Use a path that doesn't exist – Run will fail immediately and nothing is
	// stored in the cache.
	svc := NewXpumd("/tmp/nonexistent-xpumd.sock")

	_, err := svc.GetDeviceHealth("0000:00:00.0")
	if !errors.Is(err, ErrNoHealthData) {
		t.Errorf("expected ErrNoHealthData, got %v", err)
	}
}

func TestGetDeviceHealth_CriticalUnhealthy(t *testing.T) {
	const bdf = "0000:05:00.0"

	resp := createHealthResponse(bdf, xpumapi.SeverityLevel_SEVERITY_LEVEL_CRITICAL, "pcie-error")

	sockPath := startMockServer(t, []*xpumapi.DeviceHealthResponse{resp})

	svc := NewXpumd(sockPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		svc.Run(ctx)
	}()

	// Wait until health data for the device is populated.
	healthy, err := retrieveDeviceHealth(bdf, svc)

	if err != nil {
		t.Fatalf("GetDeviceHealth returned error: %v", err)
	}

	if healthy {
		t.Errorf("expected device to be unhealthy for CRITICAL severity, got healthy")
	}
}
