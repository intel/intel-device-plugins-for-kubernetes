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
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type serverState int

// Server state.
const (
	uninitialized serverState = iota
	serving
	terminating
)

// devicePluginServer maintains a gRPC server satisfying
// pluginapi.PluginInterfaceServer interfaces.
// This internal unexposed interface simplifies unit testing.
type devicePluginServer interface {
	Serve(namespace string) error
	Stop() error
	Update(devices map[string]DeviceInfo)
}

// server implements devicePluginServer and pluginapi.PluginInterfaceServer interfaces.
type server struct {
	grpcServer             *grpc.Server
	updatesCh              chan map[string]DeviceInfo
	devices                map[string]DeviceInfo
	allocate               allocateFunc
	postAllocate           postAllocateFunc
	preStartContainer      preStartContainerFunc
	getPreferredAllocation getPreferredAllocationFunc
	devType                string
	state                  serverState
	stateMutex             sync.Mutex
}

// newServer creates a new server satisfying the devicePluginServer interface.
func newServer(devType string,
	postAllocate postAllocateFunc,
	preStartContainer preStartContainerFunc,
	getPreferredAllocation getPreferredAllocationFunc,
	allocate allocateFunc) devicePluginServer {
	return &server{
		devType:                devType,
		updatesCh:              make(chan map[string]DeviceInfo, 1), // TODO: is 1 needed?
		devices:                make(map[string]DeviceInfo),
		allocate:               allocate,
		postAllocate:           postAllocate,
		preStartContainer:      preStartContainer,
		getPreferredAllocation: getPreferredAllocation,
		state:                  uninitialized,
	}
}

func (srv *server) getDevicePluginOptions() *pluginapi.DevicePluginOptions {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired:                srv.preStartContainer != nil,
		GetPreferredAllocationAvailable: srv.getPreferredAllocation != nil,
	}
}

func (srv *server) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return srv.getDevicePluginOptions(), nil
}

func (srv *server) sendDevices(stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	resp := new(pluginapi.ListAndWatchResponse)
	for id, device := range srv.devices {
		resp.Devices = append(resp.Devices, &pluginapi.Device{
			ID:       id,
			Health:   device.state,
			Topology: device.topology,
		})
	}

	klog.V(4).Info("Sending to kubelet", resp.Devices)

	if err := stream.Send(resp); err != nil {
		_ = srv.Stop()
		return errors.Wrapf(err, "Cannot update device list")
	}

	return nil
}

func (srv *server) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	klog.V(4).Info("Started ListAndWatch for", srv.devType)

	if err := srv.sendDevices(stream); err != nil {
		return err
	}

	for srv.devices = range srv.updatesCh {
		if err := srv.sendDevices(stream); err != nil {
			return err
		}
	}

	return nil
}

func (srv *server) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	if srv.allocate != nil {
		response, err := srv.allocate(rqt)

		if _, ok := err.(*UseDefaultMethodError); !ok {
			return response, err
		}
	}

	response := new(pluginapi.AllocateResponse)

	for _, crqt := range rqt.ContainerRequests {
		cresp := new(pluginapi.ContainerAllocateResponse)

		for _, id := range crqt.DevicesIDs {
			dev, ok := srv.devices[id]
			if !ok {
				return nil, errors.Errorf("Invalid allocation request with non-existing device %s", id)
			}

			if dev.state != pluginapi.Healthy {
				return nil, errors.Errorf("Invalid allocation request with unhealthy device %s", id)
			}

			for i := range dev.nodes {
				cresp.Devices = append(cresp.Devices, &dev.nodes[i])
			}

			for i := range dev.mounts {
				cresp.Mounts = append(cresp.Mounts, &dev.mounts[i])
			}

			for key, value := range dev.envs {
				if cresp.Envs == nil {
					cresp.Envs = make(map[string]string)
				}

				cresp.Envs[key] = value
			}
		}

		response.ContainerResponses = append(response.ContainerResponses, cresp)
	}

	if srv.postAllocate != nil {
		err := srv.postAllocate(response)
		if err != nil {
			return nil, err
		}
	}

	return response, nil
}

func (srv *server) PreStartContainer(ctx context.Context, rqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	if srv.preStartContainer != nil {
		return new(pluginapi.PreStartContainerResponse), srv.preStartContainer(rqt)
	}

	return nil, errors.New("PreStartContainer() should not be called as this device plugin doesn't implement it")
}

func (srv *server) GetPreferredAllocation(ctx context.Context, rqt *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	if srv.getPreferredAllocation != nil {
		return srv.getPreferredAllocation(rqt)
	}

	return nil, errors.New("GetPreferredAllocation should not be called as this device plugin doesn't implement it")
}

// Serve starts a gRPC server to serve pluginapi.PluginInterfaceServer interface.
func (srv *server) Serve(namespace string) error {
	return srv.setupAndServe(namespace, pluginapi.DevicePluginPath, pluginapi.KubeletSocket)
}

// Stop stops serving pluginapi.PluginInterfaceServer interface.
func (srv *server) Stop() error {
	if srv.grpcServer == nil {
		return errors.New("Can't stop non-existing gRPC server. Calling Stop() before Serve()?")
	}

	srv.setState(terminating)
	srv.grpcServer.Stop()
	close(srv.updatesCh)

	return nil
}

// Update sends updates from Manager to ListAndWatch's event loop.
func (srv *server) Update(devices map[string]DeviceInfo) {
	srv.updatesCh <- devices
}

func (srv *server) setState(state serverState) {
	srv.stateMutex.Lock()
	defer srv.stateMutex.Unlock()
	srv.state = state
}

func (srv *server) getState() serverState {
	srv.stateMutex.Lock()
	defer srv.stateMutex.Unlock()

	return srv.state
}

// setupAndServe binds given gRPC server to device manager, starts it and registers it with kubelet.
func (srv *server) setupAndServe(namespace string, devicePluginPath string, kubeletSocket string) error {
	resourceName := namespace + "/" + srv.devType
	pluginPrefix := namespace + "-" + srv.devType
	srv.setState(serving)

	for srv.getState() == serving {
		pluginEndpoint := pluginPrefix + ".sock"
		pluginSocket := path.Join(devicePluginPath, pluginEndpoint)

		if err := waitForServer(pluginSocket, time.Second); err == nil {
			return errors.Errorf("Socket %s is already in use", pluginSocket)
		}
		// We don't care if the plugin's socket file doesn't exist.
		_ = os.Remove(pluginSocket)

		lis, err := net.Listen("unix", pluginSocket)
		if err != nil {
			return errors.Wrap(err, "Failed to listen to plugin socket")
		}

		srv.grpcServer = grpc.NewServer()
		pluginapi.RegisterDevicePluginServer(srv.grpcServer, srv)

		// Starts device plugin service.
		go func() {
			klog.V(1).Infof("Start server for %s at: %s", srv.devType, pluginSocket)

			if serveErr := srv.grpcServer.Serve(lis); serveErr != nil {
				klog.Errorf("unable to start gRPC server: %+v", serveErr)
			}
		}()

		// Wait for the server to start
		if err = waitForServer(pluginSocket, 10*time.Second); err != nil {
			return err
		}

		// Register with Kubelet.
		err = srv.registerWithKubelet(kubeletSocket, pluginEndpoint, resourceName)
		if err != nil {
			return err
		}

		klog.V(1).Infof("Device plugin for %s registered", srv.devType)

		// Kubelet removes plugin socket when it (re)starts
		// plugin must restart in this case
		if err = watchFile(pluginSocket); err != nil {
			return err
		}

		if srv.getState() == serving {
			srv.grpcServer.Stop()
			klog.V(1).Infof("Socket %s removed, restarting", pluginSocket)
		} else {
			klog.V(1).Infof("Socket %s shut down", pluginSocket)
		}
	}

	return nil
}

func watchFile(file string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrapf(err, "Failed to create watcher for %s", file)
	}
	defer watcher.Close()

	err = watcher.Add(filepath.Dir(file))
	if err != nil {
		return errors.Wrapf(err, "Failed to add %s to watcher", file)
	}

	for {
		select {
		case ev := <-watcher.Events:
			if (ev.Op == fsnotify.Remove || ev.Op == fsnotify.Rename) && ev.Name == file {
				return nil
			}
		case err := <-watcher.Errors:
			return errors.WithStack(err)
		}
	}
}

func (srv *server) registerWithKubelet(kubeletSocket, pluginEndPoint, resourceName string) error {
	ctx := context.Background()

	conn, err := grpc.DialContext(ctx, kubeletSocket, grpc.WithInsecure(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		}))
	if err != nil {
		return errors.Wrap(err, "Cannot connect to kubelet service")
	}

	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     pluginEndPoint,
		ResourceName: resourceName,
		Options:      srv.getDevicePluginOptions(),
	}

	_, err = client.Register(ctx, reqt)
	if err != nil {
		return errors.Wrap(err, "Cannot register to kubelet service")
	}

	return nil
}

// waitForServer checks if grpc server is alive
// by making grpc blocking connection to the server socket.
func waitForServer(socket string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()

	conn, err := grpc.DialContext(ctx, socket, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", addr)
		}),
	)
	if conn != nil {
		_ = conn.Close()
	}

	return errors.Wrapf(err, "Failed dial context at %s", socket)
}
