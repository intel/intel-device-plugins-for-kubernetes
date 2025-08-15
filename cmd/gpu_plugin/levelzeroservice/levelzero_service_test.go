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
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"

	lz "github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/levelzero"
	"google.golang.org/grpc"
)

const (
	NoError       = iota
	InternalError = iota
	ExternalError = iota
)

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

type mockServer struct {
	lz.UnimplementedLevelzeroServer
	failRequest int
}

func (m *mockServer) serve(socketPath string) {
	var lc net.ListenConfig

	lis, err := lc.Listen(context.Background(), "unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	lz.RegisterLevelzeroServer(s, m)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
}

func (m *mockServer) GetDeviceHealth(c context.Context, deviceid *lz.DeviceId) (*lz.DeviceHealth, error) {
	if m.failRequest == ExternalError {
		return nil, os.ErrInvalid
	}

	health := &lz.DeviceHealth{
		BusOk:    true,
		MemoryOk: true,
		Error:    nil,
	}

	if m.failRequest == InternalError {
		health.MemoryOk = false
		health.Error = &lz.Error{
			Description: "error error",
			Errorcode:   99,
		}
	}

	return health, nil
}

func (m *mockServer) GetDeviceTemperature(c context.Context, deviceid *lz.DeviceId) (*lz.DeviceTemperature, error) {
	if m.failRequest == ExternalError {
		return nil, os.ErrInvalid
	}

	temps := &lz.DeviceTemperature{
		Global: 35.0,
		Gpu:    35.0,
		Memory: 35.0,
		Error:  nil,
	}

	if m.failRequest == InternalError {
		temps.Global = -999.0
		temps.Gpu = -999.0
		temps.Memory = -999.0
		temps.Error = &lz.Error{
			Description: "error error",
			Errorcode:   99,
		}
	}

	return temps, nil
}

func (m *mockServer) GetIntelIndices(c context.Context, msg *lz.GetIntelIndicesMessage) (*lz.DeviceIndices, error) {
	if m.failRequest == ExternalError {
		return nil, os.ErrInvalid
	}

	ret := lz.DeviceIndices{
		Indices: []uint32{0},
		Error:   nil,
	}

	if m.failRequest == InternalError {
		ret.Indices = []uint32{}
		ret.Error = &lz.Error{
			Description: "error error",
			Errorcode:   99,
		}
	}

	return &ret, nil
}

func (m *mockServer) GetDeviceMemoryAmount(c context.Context, deviceid *lz.DeviceId) (*lz.DeviceMemoryAmount, error) {
	if m.failRequest == ExternalError {
		return nil, os.ErrInvalid
	}

	ret := lz.DeviceMemoryAmount{
		MemorySize: 1000,
		Error:      nil,
	}

	if m.failRequest == InternalError {
		ret.MemorySize = 0
		ret.Error = &lz.Error{
			Description: "error error",
			Errorcode:   99,
		}
	}

	return &ret, nil
}

type testcase struct {
	name string
	fail int
}

var tcases = []testcase{
	{
		name: "normal flow",
		fail: NoError,
	},
	{
		name: "fail flow - internal",
		fail: InternalError,
	},
	{
		name: "fail flow - external",
		fail: ExternalError,
	},
}

func TestGetDeviceHealth(t *testing.T) {
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := os.MkdirTemp("", "testinglevelzero*")
			if err != nil {
				t.Fatal("failed to create tmp directory")
			}

			defer os.RemoveAll(d)

			sockPath := filepath.Join(d, "server.sock")

			mock := mockServer{
				failRequest: tc.fail,
			}

			mock.serve(sockPath)

			n := NewLevelzero(sockPath)

			n.Run(false)

			dh, err := n.GetDeviceHealth("0000:00:00.1")

			if tc.fail == NoError && err != nil {
				t.Error("GetDeviceHealth returned an error:", err)
			}

			if tc.fail == NoError && (!dh.Memory || !dh.Bus) {
				t.Error("Call to device health returned unhealthy", dh, tc.fail)
			}

			if tc.fail == ExternalError && err == nil {
				t.Error("GetDeviceHealth returned nil and expected error")
			}
		})
	}
}

func TestGetIndices(t *testing.T) {
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := os.MkdirTemp("", "testinglevelzero*")
			if err != nil {
				t.Fatal("failed to create tmp directory")
			}

			defer os.RemoveAll(d)

			sockPath := filepath.Join(d, "server.sock")

			mock := mockServer{
				failRequest: tc.fail,
			}

			mock.serve(sockPath)

			n := NewLevelzero(sockPath)

			n.Run(false)

			indices, err := n.GetIntelIndices()

			if tc.fail == NoError && err != nil {
				t.Error("GetIntelIndices returned error:", err)
			}

			if tc.fail == ExternalError && err == nil {
				t.Error("GetIntelIndices returned nil and expected error")
			}

			if tc.fail == NoError && len(indices) != 1 {
				t.Error("Wrong number of indices received", indices)
			}
			if tc.fail != NoError && len(indices) != 0 {
				t.Error("Wrong number of indices received", indices)
			}
		})
	}
}

func TestGetMemoryAmount(t *testing.T) {
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := os.MkdirTemp("", "testinglevelzero*")
			if err != nil {
				t.Fatal("failed to create tmp directory")
			}

			defer os.RemoveAll(d)

			sockPath := filepath.Join(d, "server.sock")

			mock := mockServer{
				failRequest: tc.fail,
			}

			mock.serve(sockPath)

			n := NewLevelzero(sockPath)
			n.Run(false)

			memSize, err := n.GetDeviceMemoryAmount("0000:11:22.3")

			if tc.fail == NoError && err != nil {
				t.Error("TestGetMemoryAmount returned error:", err)
			}

			if tc.fail == ExternalError && err == nil {
				t.Error("TestGetMemoryAmount returned nil and expected error")
			}

			if tc.fail == NoError && memSize != 1000 {
				t.Error("Wrong mem size received", memSize)
			}
		})
	}
}

func TestAccessBeforeReady(t *testing.T) {
	n := NewLevelzero("/tmp/foobar.sock")

	_, err := n.GetDeviceMemoryAmount("")
	if err == nil {
		t.Error("Got non-error for memory amount, expected error")
	}

	_, err = n.GetDeviceHealth("")
	if err == nil {
		t.Error("Got non-error for health, expected error")
	}

	_, err = n.GetIntelIndices()
	if err == nil {
		t.Error("Got non-error for indices, expected error")
	}
}
