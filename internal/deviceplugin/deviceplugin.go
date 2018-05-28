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
	"context"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"google.golang.org/grpc"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

// DeviceInfo contains information about device maintained by Device Plugin
type DeviceInfo struct {
	State string
	Nodes []string
}

// Server structure to keep server parameters
type Server struct {
	grpcServer *grpc.Server
}

// Serve serves starts gRPC server to serve Device Plugin functionality
func (srv *Server) Serve(dm pluginapi.DevicePluginServer, resourceName string, pluginPrefix string) error {
	return srv.setupAndServe(dm, resourceName, pluginPrefix, pluginapi.DevicePluginPath, pluginapi.KubeletSocket)
}

// Stop stops serving Device Plugin
func (srv *Server) Stop() error {
	if srv.grpcServer == nil {
		return fmt.Errorf("Can't stop non-existing gRPC server. Calling Stop() before Serve()?")
	}
	srv.grpcServer.Stop()
	return nil
}

// setupAndServe binds given gRPC server to device manager, starts it and registers it with kubelet.
func (srv *Server) setupAndServe(dm pluginapi.DevicePluginServer, resourceName string, pluginPrefix string, devicePluginPath string, kubeletSocket string) error {
	for {
		pluginEndpoint := pluginPrefix + ".sock"
		pluginSocket := path.Join(devicePluginPath, pluginEndpoint)

		if err := waitForServer(pluginSocket, time.Second); err == nil {
			return fmt.Errorf("Socket %s is already in use", pluginSocket)
		}
		os.Remove(pluginSocket)

		lis, err := net.Listen("unix", pluginSocket)
		if err != nil {
			return fmt.Errorf("Failed to listen to plugin socket: %+v", err)
		}

		srv.grpcServer = grpc.NewServer()
		pluginapi.RegisterDevicePluginServer(srv.grpcServer, dm)

		// Starts device plugin service.
		go func() {
			fmt.Printf("device-plugin start server at: %s\n", pluginSocket)
			srv.grpcServer.Serve(lis)
		}()

		// Wait for the server to start
		if err = waitForServer(pluginSocket, 10*time.Second); err != nil {
			return fmt.Errorf("Failed to wait for plugin socket: %+v", err)
		}

		// Register with Kubelet.
		err = registerWithKubelet(kubeletSocket, pluginEndpoint, resourceName)
		if err != nil {
			return fmt.Errorf("Failed to register: %+v", err)
		}
		fmt.Println("device-plugin registered")

		// Kubelet removes plugin socket when it (re)starts
		// plugin must restart in this case
		for {
			if _, err := os.Stat(pluginSocket); os.IsNotExist(err) {
				fmt.Println("plugin socket removed, stop server")
				srv.grpcServer.Stop()
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func registerWithKubelet(kubeletSocket, pluginEndPoint, resourceName string) error {
	conn, err := grpc.Dial(kubeletSocket, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	defer conn.Close()
	if err != nil {
		return fmt.Errorf("device-plugin: cannot connect to kubelet service: %v", err)
	}
	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     pluginEndPoint,
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return fmt.Errorf("device-plugin: cannot register to kubelet service: %v", err)
	}

	return nil
}

// waitForServer checks if grpc server is alive
// by making grpc blocking connection to the server socket
func waitForServer(socket string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, socket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)
	if conn != nil {
		conn.Close()
	}
	return err
}

// MakeAllocateResponse creates response data for Allocate GRPC call
func MakeAllocateResponse(rqt *pluginapi.AllocateRequest, devices map[string]DeviceInfo) (*pluginapi.AllocateResponse, error) {
	resp := new(pluginapi.AllocateResponse)
	for _, crqt := range rqt.ContainerRequests {
		cresp := new(pluginapi.ContainerAllocateResponse)
		for _, id := range crqt.DevicesIDs {
			dev, ok := devices[id]
			if !ok {
				return nil, fmt.Errorf("Invalid allocation request with non-existing device %s", id)
			}
			if dev.State != pluginapi.Healthy {
				return nil, fmt.Errorf("Invalid allocation request with unhealthy device %s", id)
			}
			for _, devnode := range dev.Nodes {
				cresp.Devices = append(cresp.Devices, &pluginapi.DeviceSpec{
					HostPath:      devnode,
					ContainerPath: devnode,
					Permissions:   "mrw",
				})
			}
		}
		resp.ContainerResponses = append(resp.ContainerResponses, cresp)
	}
	return resp, nil
}
