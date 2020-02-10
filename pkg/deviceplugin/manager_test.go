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
	"testing"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

func init() {
	debug.Activate()
}

func TestNotify(t *testing.T) {
	tcases := []struct {
		name            string
		expectedAdded   int
		expectedUpdated int
		expectedRemoved int
		oldmap          map[string]map[string]DeviceInfo
		newmap          map[string]map[string]DeviceInfo
	}{
		{
			name: "No devices found",
		},
		{
			name: "Added 1 new device type",
			newmap: map[string]map[string]DeviceInfo{
				"someDeviceType": {
					"intel-fpga-port.0": {
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
			expectedAdded: 1,
		},
		{
			name: "Updated 1 new device type",
			oldmap: map[string]map[string]DeviceInfo{
				"someDeviceType": {
					"intel-fpga-port.0": {
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
			newmap: map[string]map[string]DeviceInfo{
				"someDeviceType": {
					"intel-fpga-port.1": {
						state: pluginapi.Healthy,
						nodes: []pluginapi.DeviceSpec{
							{
								HostPath:      "/dev/intel-fpga-port.1",
								ContainerPath: "/dev/intel-fpga-port.1",
								Permissions:   "rw",
							},
						},
					},
				},
			},
			expectedUpdated: 1,
		},
		{
			name: "Removed 1 new device type",
			oldmap: map[string]map[string]DeviceInfo{
				"someDeviceType": {
					"intel-fpga-port.0": {
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
			expectedRemoved: 1,
		},
	}

	for _, tcase := range tcases {
		ch := make(chan updateInfo, 1)
		n := newNotifier(ch)
		n.deviceTree = tcase.oldmap

		n.Notify(tcase.newmap)

		var update updateInfo
		select {
		case update = <-ch:
		default:
		}

		if tcase.expectedAdded != len(update.Added) {
			t.Errorf("Test case '%s': expected %d added device types, but got %d", tcase.name, tcase.expectedAdded, len(update.Added))
		}
		if tcase.expectedUpdated != len(update.Updated) {
			t.Errorf("Test case '%s': expected %d updated device types, but got %d", tcase.name, tcase.expectedUpdated, len(update.Updated))
		}
		if tcase.expectedRemoved != len(update.Removed) {
			t.Errorf("Test case '%s': expected %d removed device types, but got %d", tcase.name, tcase.expectedUpdated, len(update.Updated))
		}
	}
}

type serverStub struct{}

func (*serverStub) Serve(string) error {
	return nil
}

func (*serverStub) Update(map[string]DeviceInfo) {}

func (*serverStub) Stop() error {
	return nil
}

type devicePluginStub struct{}

func (*devicePluginStub) Scan(n Notifier) error {
	tree := NewDeviceTree()
	tree.AddDevice("testdevice", "dev1", DeviceInfo{
		state: pluginapi.Healthy,
		nodes: make([]pluginapi.DeviceSpec, 0),
	})
	n.Notify(tree)
	return nil
}

func (*devicePluginStub) PostAllocate(*pluginapi.AllocateResponse) error {
	return nil
}

func TestHandleUpdate(t *testing.T) {
	tcases := []struct {
		name            string
		servers         map[string]devicePluginServer
		update          updateInfo
		expectedServers int
	}{
		{
			name:            "Empty update",
			update:          updateInfo{},
			expectedServers: 0,
		},
		{
			name: "Add device manager",
			update: updateInfo{
				Added: map[string]map[string]DeviceInfo{
					"ce48969398f05f33946d560708be108a": {
						"intel-fpga-fme.0": {
							state: pluginapi.Healthy,
							nodes: []pluginapi.DeviceSpec{
								{
									HostPath:      "/dev/intel-fpga-port.0",
									ContainerPath: "/dev/intel-fpga-port.0",
									Permissions:   "rw",
								},
								{
									HostPath:      "/dev/intel-fpga-fme.0",
									ContainerPath: "/dev/intel-fpga-fme.0",
									Permissions:   "rw",
								},
							},
						},
						"intel-fpga-fme.1": {
							state: pluginapi.Healthy,
							nodes: []pluginapi.DeviceSpec{
								{
									HostPath:      "/dev/intel-fpga-port.1",
									ContainerPath: "/dev/intel-fpga-port.1",
									Permissions:   "rw",
								},
								{
									HostPath:      "/dev/intel-fpga-fme.1",
									ContainerPath: "/dev/intel-fpga-fme.1",
									Permissions:   "rw",
								},
							},
						},
					},
				},
			},
			expectedServers: 1,
		},
		{
			name: "Update existing device manager",
			servers: map[string]devicePluginServer{
				"ce48969398f05f33946d560708be108a": &serverStub{},
			},
			update: updateInfo{
				Updated: map[string]map[string]DeviceInfo{
					"ce48969398f05f33946d560708be108a": {
						"intel-fpga-fme.1": {
							state: pluginapi.Healthy,
							nodes: []pluginapi.DeviceSpec{
								{
									HostPath:      "/dev/intel-fpga-port.1",
									ContainerPath: "/dev/intel-fpga-port.1",
									Permissions:   "rw",
								},
								{
									HostPath:      "/dev/intel-fpga-fme.1",
									ContainerPath: "/dev/intel-fpga-fme.1",
									Permissions:   "rw",
								},
							},
						},
					},
				},
			},
			expectedServers: 1,
		},
		{
			name: "Remove device manager",
			servers: map[string]devicePluginServer{
				"ce48969398f05f33946d560708be108a": &serverStub{},
			},
			update: updateInfo{
				Removed: map[string]map[string]DeviceInfo{
					"ce48969398f05f33946d560708be108a": {},
				},
			},
			expectedServers: 0,
		},
	}

	for _, tt := range tcases {
		if tt.servers == nil {
			tt.servers = make(map[string]devicePluginServer)
		}
		mgr := Manager{
			devicePlugin: &devicePluginStub{},
			servers:      tt.servers,
			createServer: func(string, func(*pluginapi.AllocateResponse) error) devicePluginServer {
				return &serverStub{}
			},
		}
		mgr.handleUpdate(tt.update)
		if len(tt.servers) != tt.expectedServers {
			t.Errorf("Test case '%s': expected %d runnig device managers, but got %d",
				tt.name, tt.expectedServers, len(tt.servers))
		}
	}
}

func TestRun(t *testing.T) {
	mgr := NewManager("testnamespace", &devicePluginStub{})
	mgr.createServer = func(string, func(*pluginapi.AllocateResponse) error) devicePluginServer {
		return &serverStub{}
	}
	mgr.Run()
}
