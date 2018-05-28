// Copyright 2017 Intel Corporation. All Rights Reserved.
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
	"fmt"
	"net"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	devicePluginPath = "/tmp/"
	kubeletSocket    = devicePluginPath + "kubelet-test.sock"
	pluginEndpoint   = "plugin-test.sock"
	resourceName     = "intel.com/testdev"
)

type kubeletStub struct {
	sync.Mutex
	socket         string
	pluginEndpoint string
	server         *grpc.Server
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
		fmt.Printf("Can't listen at the socket: %+v", err)
		return err
	}

	k.server = grpc.NewServer()

	pluginapi.RegisterRegistrationServer(k.server, k)
	go k.server.Serve(s)

	// Wait till the grpcServer is ready to serve services.
	waitForServer(k.socket, 10*time.Second)

	return nil
}

type pluginStub struct {
}

// Minimal implementation of pluginapi.DevicePluginServer

func (ps *pluginStub) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return new(pluginapi.DevicePluginOptions), nil
}

func (ps *pluginStub) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	return nil
}

func (ps *pluginStub) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	fmt.Println("Called fake Allocate")
	resp := new(pluginapi.AllocateResponse)
	return resp, nil
}

func (ps *pluginStub) PreStartContainer(ctx context.Context, rqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return new(pluginapi.PreStartContainerResponse), nil
}

func TestRegisterWithKublet(t *testing.T) {
	pluginSocket := path.Join(devicePluginPath, pluginEndpoint)

	err := registerWithKubelet(kubeletSocket, pluginSocket, resourceName)
	if err == nil {
		t.Error("No error triggered when kubelet is not accessible")
	}

	kubelet := newKubeletStub(kubeletSocket)
	kubelet.start()
	defer kubelet.server.Stop()

	err = registerWithKubelet(kubeletSocket, pluginSocket, resourceName)
	if err != nil {
		t.Errorf("Can't register device plugin: %+v", err)
	}
}

func TestSetupAndServe(t *testing.T) {
	var pluginSocket string
	var pEndpoint string
	var srv Server

	kubelet := newKubeletStub(kubeletSocket)
	kubelet.start()
	defer kubelet.server.Stop()

	testPlugin := &pluginStub{}
	defer srv.Stop()

	go srv.setupAndServe(testPlugin, resourceName, "testPlugin", devicePluginPath, kubeletSocket)

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

	err := srv.setupAndServe(testPlugin, resourceName, "testPlugin", devicePluginPath, kubeletSocket)
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
		fmt.Println("No plugin socket. Waiting...")
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
	srv := &Server{}
	if err := srv.Stop(); err == nil {
		t.Error("Calling Stop() before Serve() is successful")
	}
}

func TestMakeAllocateResponse(t *testing.T) {
	rqt := &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{
				DevicesIDs: []string{"dev1"},
			},
		},
	}

	_, err := MakeAllocateResponse(rqt, nil)
	if err == nil {
		t.Fatal("No error when allocating non-existing device")
	}

	devices := map[string]DeviceInfo{
		"dev1": {pluginapi.Unhealthy, []string{"/dev/dev1"}},
	}

	_, err = MakeAllocateResponse(rqt, devices)
	if err == nil {
		t.Fatal("No error when allocating unhealthy device")
	}

	devices["dev1"] = DeviceInfo{pluginapi.Healthy, []string{"/dev/dev1"}}
	resp, err := MakeAllocateResponse(rqt, devices)
	if err != nil {
		t.Fatalf("Failed to allocate healthy device: %+v", err)
	}

	if len(resp.ContainerResponses[0].Devices) != 1 {
		t.Fatal("Allocated wrong number of devices")
	}
}
