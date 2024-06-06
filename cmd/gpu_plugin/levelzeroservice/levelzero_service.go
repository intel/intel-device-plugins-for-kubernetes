// Copyright 2024 Intel Corporation. All Rights Reserved.
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

package levelzeroservice

import (
	"context"

	lz "github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/levelzero"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/klog/v2"
)

type LevelzeroService interface {
	Run(bool)
	GetIntelIndices() ([]uint32, error)
	GetDeviceHealth(bdfAddress string) (DeviceHealth, error)
	GetDeviceTemperature(bdfAddress string) (DeviceTemperature, error)
	GetDeviceMemoryAmount(bdfAddress string) (uint64, error)
}

type DeviceHealth struct {
	Memory            bool
	Bus               bool
	SoC               bool
	GlobalTemperature float64
	GPUTemperature    float64
	MemoryTemperature float64
}

type DeviceTemperature struct {
	Global float64
	GPU    float64
	Memory float64
}

type clientNotReadyErr struct{}

func (e *clientNotReadyErr) Error() string {
	return "client is not (yet) ready"
}

func NewLevelzero(socket string) LevelzeroService {
	return &levelzero{
		socketPath: socket,
		ctx:        context.Background(),
		conn:       nil,
		client:     nil,
	}
}

type levelzero struct {
	client     lz.LevelzeroClient
	ctx        context.Context
	conn       *grpc.ClientConn
	socketPath string
}

func (l *levelzero) Run(keep bool) {
	url := "unix://" + l.socketPath

	klog.V(3).Info("Starting Level-Zero client. Connecting to: ", url)

	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		klog.Error("Failed to connect to socket", err)

		return
	}

	ctx := context.Background()

	l.conn = conn

	for {
		state := l.conn.GetState()
		if state == connectivity.Idle {
			conn.Connect()
		}

		if state == connectivity.Ready {
			klog.V(2).Info("Connection ready")

			l.client = lz.NewLevelzeroClient(conn)

			if !keep {
				break
			}
		}

		if !conn.WaitForStateChange(ctx, state) {
			continue
		}
	}
}

func (l *levelzero) isClientReady() bool {
	return l.client != nil
}

func (l *levelzero) GetIntelIndices() ([]uint32, error) {
	if !l.isClientReady() {
		return []uint32{}, &clientNotReadyErr{}
	}

	cli := l.client

	indices, err := cli.GetIntelIndices(l.ctx, &lz.GetIntelIndicesMessage{})
	if err != nil || indices == nil {
		return []uint32{}, err
	}

	if indices.Error != nil && indices.Error.Errorcode != 0 {
		klog.Warningf("indices request returned internal error: 0x%X (%s)", indices.Error.Errorcode, indices.Error.Description)
	}

	return indices.Indices, nil
}

func (l *levelzero) GetDeviceHealth(bdfAddress string) (DeviceHealth, error) {
	if !l.isClientReady() {
		return DeviceHealth{}, &clientNotReadyErr{}
	}

	cli := l.client

	did := lz.DeviceId{
		BdfAddress: bdfAddress,
	}

	health, err := cli.GetDeviceHealth(l.ctx, &did)
	if err != nil || health == nil {
		return DeviceHealth{}, err
	}

	if health.Error != nil && health.Error.Errorcode != 0 {
		klog.Warningf("health request returned internal error: 0x%X (%s)", health.Error.Errorcode, health.Error.Description)
	}

	return DeviceHealth{
		Memory: health.MemoryOk,
		Bus:    health.BusOk,
		SoC:    health.SocOk,
	}, nil
}

func (l *levelzero) GetDeviceTemperature(bdfAddress string) (DeviceTemperature, error) {
	if !l.isClientReady() {
		return DeviceTemperature{}, &clientNotReadyErr{}
	}

	cli := l.client

	did := lz.DeviceId{
		BdfAddress: bdfAddress,
	}

	temps, err := cli.GetDeviceTemperature(l.ctx, &did)
	if err != nil || temps == nil {
		return DeviceTemperature{}, err
	}

	if temps.Error != nil && temps.Error.Errorcode != 0 {
		klog.Warningf("temperature request returned internal error: 0x%X (%s)", temps.Error.Errorcode, temps.Error.Description)
	}

	return DeviceTemperature{
		Global: temps.Global,
		GPU:    temps.Gpu,
		Memory: temps.Memory,
	}, nil
}

func (l *levelzero) GetDeviceMemoryAmount(bdfAddress string) (uint64, error) {
	if !l.isClientReady() {
		return 0, &clientNotReadyErr{}
	}

	cli := l.client

	did := lz.DeviceId{
		BdfAddress: bdfAddress,
	}

	memSize, err := cli.GetDeviceMemoryAmount(l.ctx, &did)
	if err != nil || memSize == nil {
		return 0, err
	}

	if memSize.Error != nil && memSize.Error.Errorcode != 0 {
		klog.Warningf("memory request returned internal error: 0x%X (%s)", memSize.Error.Errorcode, memSize.Error.Description)
	}

	return memSize.MemorySize, nil
}
