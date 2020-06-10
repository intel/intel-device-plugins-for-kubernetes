// Copyright 2018 Intel Corporation. All Rights Reserved.
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

package deviceplugin

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"reflect"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/pkg/errors"
)

const (
	devicePluginPath = "/tmp/"
	kubeletSocket    = devicePluginPath + "kubelet-test.sock"
	namespace        = "test.intel.com"
	pluginEndpoint   = namespace + "-testdevicetype.sock"
	resourceName     = namespace + "/testdevicetype"
)

type kubeletStub struct {
	sync.Mutex
	socket         string
	pluginEndpoint string
	server         *grpc.Server
}

func init() {
	flag.Set("v", "4") //Enable debug output
}

// newKubeletStub returns an initialized kubeletStub for testing purpose.
func newKubeletStub(socket string) *kubeletStub {
	return &kubeletStub{
		socket: socket,
	}
}

// Minimal implementation of deviceplugin.RegistrationServer interface

func (k *kubeletStub) Register(ctx context.Context, r *pluginapi.RegisterRequest) (*pluginapi.Empty, error) {
	k.Lock()
	defer k.Unlock()
	k.pluginEndpoint = r.Endpoint
	return &pluginapi.Empty{}, nil
}

func (k *kubeletStub) start() error {
	os.Remove(k.socket)
	s, err := net.Listen("unix", k.socket)
	if err != nil {
		return errors.Wrap(err, "Can't listen at the socket")
	}

	k.server = grpc.NewServer()

	pluginapi.RegisterRegistrationServer(k.server, k)
	go k.server.Serve(s)

	// Wait till the grpcServer is ready to serve services.
	waitForServer(k.socket, 10*time.Second)

	return nil
}

func TestRegisterWithKublet(t *testing.T) {
	pluginSocket := path.Join(devicePluginPath, pluginEndpoint)

	err := registerWithKubelet(kubeletSocket, pluginSocket, resourceName, nil)
	if err == nil {
		t.Error("No error triggered when kubelet is not accessible")
	}

	kubelet := newKubeletStub(kubeletSocket)
	err = kubelet.start()
	if err != nil {
		t.Fatalf("%+v", err)
	}
	defer kubelet.server.Stop()

	err = registerWithKubelet(kubeletSocket, pluginSocket, resourceName, nil)
	if err != nil {
		t.Errorf("Can't register device plugin: %+v", err)
	}
}

func TestSetupAndServe(t *testing.T) {
	var pluginSocket string
	var pEndpoint string

	kubelet := newKubeletStub(kubeletSocket)
	kubelet.start()
	defer kubelet.server.Stop()

	srv := &server{
		devType: "testtype",
		devices: map[string]DeviceInfo{
			"dev1": {
				state: pluginapi.Healthy,
			},
			"dev2": {
				state: pluginapi.Healthy,
			},
		},
		updatesCh: make(chan map[string]DeviceInfo),
	}

	defer srv.Stop()
	go srv.setupAndServe(namespace, devicePluginPath, kubeletSocket)

	// Wait till the grpcServer is ready to serve services.
	for {
		kubelet.Lock()
		pEndpoint = kubelet.pluginEndpoint
		kubelet.Unlock()
		pluginSocket = path.Join(devicePluginPath, pEndpoint)
		if pEndpoint != "" {
			if _, err := os.Stat(pluginSocket); err == nil {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	err := srv.setupAndServe(namespace, devicePluginPath, kubeletSocket)
	if err == nil {
		t.Fatalf("Server was able to start on occupied socket %s: %+v", pluginSocket, err)
	}

	conn, err := grpc.Dial(pluginSocket, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		t.Fatalf("Failed to get connection: %+v", err)
	}

	client := pluginapi.NewDevicePluginClient(conn)
	_, err = client.Allocate(context.Background(), &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{
				DevicesIDs: []string{"dev1", "dev2"},
			},
		},
	})
	if err != nil {
		t.Errorf("Failed to allocate device dev1: %+v", err)
	}
	conn.Close()

	// Check if plugins re-registers after its socket has been removed
	kubelet.Lock()
	pEndpoint = kubelet.pluginEndpoint
	kubelet.Unlock()
	if pEndpoint == "" {
		t.Fatal("After successful Allocate() pluginEndpoint is empty")
	}
	os.Remove(path.Join(devicePluginPath, pEndpoint))
	for {
		kubelet.Lock()
		pEndpoint = kubelet.pluginEndpoint
		kubelet.Unlock()
		pluginSocket = path.Join(devicePluginPath, pEndpoint)
		if pEndpoint != "" {
			if _, err = os.Stat(pluginSocket); err == nil {
				break
			}
		}
		klog.V(1).Info("No plugin socket. Waiting...")
		time.Sleep(1 * time.Second)
	}
	conn, err = grpc.Dial(pluginSocket, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	if err != nil {
		t.Fatalf("Failed to get connection: %+v", err)
	}

	client = pluginapi.NewDevicePluginClient(conn)
	_, err = client.Allocate(context.Background(), &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{
				DevicesIDs: []string{"dev1", "dev2"},
			},
		},
	})
	if err != nil {
		t.Errorf("Failed to allocate device dev1: %+v", err)
	}
	conn.Close()
}

func TestStop(t *testing.T) {
	srv := &server{}
	if err := srv.Stop(); err == nil {
		t.Error("Calling Stop() before Serve() is successful")
	}
}

func TestAllocate(t *testing.T) {
	rqt := &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{
				DevicesIDs: []string{"dev1"},
			},
		},
	}
	srv := &server{}

	tcases := []struct {
		name              string
		devices           map[string]DeviceInfo
		postAllocate      func(*pluginapi.AllocateResponse) error
		expectedAllocated int
		expectedErr       bool
	}{
		{
			name:        "Allocate non-existing device",
			expectedErr: true,
		},
		{
			name: "Allocate unhealthy devices",
			devices: map[string]DeviceInfo{
				"dev1": {
					state: pluginapi.Unhealthy,
					nodes: []pluginapi.DeviceSpec{
						{
							HostPath:      "/dev/dev1",
							ContainerPath: "/dev/dev1",
							Permissions:   "rw",
						},
					},
				},
			},
			expectedErr: true,
		},
		{
			name: "Allocate healthy device",
			devices: map[string]DeviceInfo{
				"dev1": {
					state: pluginapi.Healthy,
					nodes: []pluginapi.DeviceSpec{
						{
							HostPath:      "/dev/dev1",
							ContainerPath: "/dev/dev1",
							Permissions:   "rw",
						},
					},
				},
			},
			expectedAllocated: 1,
		},
		{
			name: "Allocate healthy device with postAllocate hook",
			devices: map[string]DeviceInfo{
				"dev1": {
					state: pluginapi.Healthy,
					nodes: []pluginapi.DeviceSpec{
						{
							HostPath:      "/dev/dev1",
							ContainerPath: "/dev/dev1",
							Permissions:   "rw",
						},
						{
							HostPath:      "/dev/dev2",
							ContainerPath: "/dev/dev2",
							Permissions:   "rw",
						},
					},
					mounts: []pluginapi.Mount{
						{
							HostPath:      "/dev",
							ContainerPath: "/dev",
						},
						{
							HostPath:      "/mnt",
							ContainerPath: "/mnt",
						},
					},
					envs: map[string]string{
						"testname": "testvalue",
					},
				},
			},
			postAllocate: func(resp *pluginapi.AllocateResponse) error {
				return nil
			},
			expectedAllocated: 2,
		},
		{
			name: "Allocate healthy device with failing postAllocate hook",
			devices: map[string]DeviceInfo{
				"dev1": {
					state: pluginapi.Healthy,
					nodes: []pluginapi.DeviceSpec{
						{
							HostPath:      "/dev/dev1",
							ContainerPath: "/dev/dev1",
							Permissions:   "rw",
						},
					},
				},
			},
			postAllocate: func(resp *pluginapi.AllocateResponse) error {
				return fmt.Errorf("Fake error for %s", "dev1")
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		srv.devices = tt.devices
		srv.postAllocate = tt.postAllocate
		resp, err := srv.Allocate(nil, rqt)

		if tt.expectedErr && err == nil {
			t.Errorf("Test case '%s': no error returned", tt.name)
			continue
		}
		if !tt.expectedErr && err != nil {
			t.Errorf("Test case '%s': got unexpected error %+v", tt.name, err)
			continue
		}
		if tt.expectedAllocated > 0 && len(resp.ContainerResponses[0].Devices) != tt.expectedAllocated {
			t.Errorf("Test case '%s': allocated wrong number of devices", tt.name)
		}
		if tt.expectedAllocated > 1 {
			if reflect.DeepEqual(resp.ContainerResponses[0].Devices[0], resp.ContainerResponses[0].Devices[1]) {
				t.Errorf("Test case '%s': got equal dev nodes in the same response", tt.name)
			}
		}
	}
}

// Minimal implementation of pluginapi.DevicePlugin_ListAndWatchServer.
type listAndWatchServerStub struct {
	testServer  *server
	generateErr int
	sendCounter int
	cdata       chan []*pluginapi.Device
}

func (s *listAndWatchServerStub) Send(resp *pluginapi.ListAndWatchResponse) error {
	s.sendCounter = s.sendCounter + 1
	if s.generateErr == s.sendCounter {
		klog.V(4).Info("listAndWatchServerStub::Send returns error")
		return fmt.Errorf("Fake error")
	}

	klog.V(4).Info("listAndWatchServerStub::Send", resp.Devices)
	s.cdata <- resp.Devices
	return nil
}

func (s *listAndWatchServerStub) Context() context.Context {
	return nil
}

func (s *listAndWatchServerStub) RecvMsg(m interface{}) error {
	return nil
}

func (s *listAndWatchServerStub) SendMsg(m interface{}) error {
	return nil
}

func (s *listAndWatchServerStub) SendHeader(m metadata.MD) error {
	return nil
}

func (s *listAndWatchServerStub) SetHeader(m metadata.MD) error {
	return nil
}

func (s *listAndWatchServerStub) SetTrailer(m metadata.MD) {
}

func TestListAndWatch(t *testing.T) {
	tcases := []struct {
		name        string
		updates     []map[string]DeviceInfo
		errorOnCall int
	}{
		{
			name: "No updates and close",
		},
		{
			name:        "No updates and close, but expect streaming error",
			errorOnCall: 1,
		},
		{
			name: "Send 1 update",
			updates: []map[string]DeviceInfo{
				{
					"fake_id": {
						state: pluginapi.Healthy,
						nodes: []pluginapi.DeviceSpec{
							{
								HostPath:      "/dev/intel-fpga-port.0",
								ContainerPath: "/dev/intel-fpga-port.0",
								Permissions:   "rw",
							},
						},
					},
				},
			},
		},
		{
			name: "Send 1 update, but expect streaming error",
			updates: []map[string]DeviceInfo{
				{
					"fake_id": {
						state: pluginapi.Healthy,
						nodes: []pluginapi.DeviceSpec{
							{
								HostPath:      "/dev/intel-fpga-port.0",
								ContainerPath: "/dev/intel-fpga-port.0",
								Permissions:   "rw",
							},
						},
					},
				},
			},
			errorOnCall: 2,
		},
	}

	for _, tt := range tcases {
		devCh := make(chan map[string]DeviceInfo, len(tt.updates))
		testServer := &server{
			updatesCh: devCh,
		}

		server := &listAndWatchServerStub{
			testServer:  testServer,
			generateErr: tt.errorOnCall,
			cdata:       make(chan []*pluginapi.Device, len(tt.updates)+1),
		}

		// push device infos to DM's channel
		for _, update := range tt.updates {
			devCh <- update
		}
		close(devCh)

		err := testServer.ListAndWatch(&pluginapi.Empty{}, server)
		if err != nil && tt.errorOnCall == 0 {
			t.Errorf("Test case '%s': got unexpected error %+v", tt.name, err)
		}
		if err == nil && tt.errorOnCall != 0 {
			t.Errorf("Test case '%s': expected an error, but got nothing", tt.name)
		}
	}
}

func TestGetDevicePluginOptions(t *testing.T) {
	srv := &server{}
	srv.GetDevicePluginOptions(nil, nil)
}

func TestPreStartContainer(t *testing.T) {
	tcases := []struct {
		name              string
		preStartContainer func(*pluginapi.PreStartContainerRequest) error
		expectedError     bool
	}{
		{
			name: "success",
			preStartContainer: func(*pluginapi.PreStartContainerRequest) error {
				return nil
			},
		},
		{
			name:          "error",
			expectedError: true,
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &server{
				preStartContainer: tc.preStartContainer,
			}
			_, err := srv.PreStartContainer(nil, nil)
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			} else if tc.expectedError && err == nil {
				t.Error("didn't failed when expected to fail")
				return
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	_ = newServer("test", nil, nil)
}

func TestUpdate(t *testing.T) {
	srv := &server{
		updatesCh: make(chan map[string]DeviceInfo, 1),
	}
	srv.Update(make(map[string]DeviceInfo))
}
