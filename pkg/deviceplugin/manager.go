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
	"os"
	"reflect"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type allocateFunc func(*pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error)
type postAllocateFunc func(*pluginapi.AllocateResponse) error
type preStartContainerFunc func(*pluginapi.PreStartContainerRequest) error
type getPreferredAllocationFunc func(*pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error)

// updateInfo contains info for added, updated and deleted devices.
type updateInfo struct {
	Added   DeviceTree
	Updated DeviceTree
	Removed DeviceTree
}

// notifier implements Notifier interface.
type notifier struct {
	deviceTree DeviceTree
	updatesCh  chan<- updateInfo
}

func newNotifier(updatesCh chan<- updateInfo) *notifier {
	return &notifier{
		updatesCh: updatesCh,
	}
}

func (n *notifier) Notify(newDeviceTree DeviceTree) {
	added := NewDeviceTree()
	updated := NewDeviceTree()

	for devType, new := range newDeviceTree {
		if old, ok := n.deviceTree[devType]; ok {
			if !reflect.DeepEqual(old, new) {
				updated[devType] = new
			}
			delete(n.deviceTree, devType)
		} else {
			added[devType] = new
		}
	}

	if len(added) > 0 || len(updated) > 0 || len(n.deviceTree) > 0 {
		n.updatesCh <- updateInfo{
			Added:   added,
			Updated: updated,
			Removed: n.deviceTree,
		}
	}

	n.deviceTree = newDeviceTree
}

// Manager manages life cycle of device plugins and handles the scan results
// received from them.
type Manager struct {
	devicePlugin Scanner
	servers      map[string]devicePluginServer
	createServer func(string, postAllocateFunc, preStartContainerFunc, getPreferredAllocationFunc, allocateFunc) devicePluginServer
	namespace    string
}

// NewManager creates a new instance of Manager.
func NewManager(namespace string, devicePlugin Scanner) *Manager {
	return &Manager{
		devicePlugin: devicePlugin,
		namespace:    namespace,
		servers:      make(map[string]devicePluginServer),
		createServer: newServer,
	}
}

// Run prepares and launches event loop for updates from Scanner.
func (m *Manager) Run() {
	updatesCh := make(chan updateInfo)

	go func() {
		err := m.devicePlugin.Scan(newNotifier(updatesCh))
		if err != nil {
			klog.Errorf("Device scan failed: %+v", err)
			os.Exit(1)
		}
		close(updatesCh)
	}()

	for update := range updatesCh {
		m.handleUpdate(update)
	}
}

func (m *Manager) handleUpdate(update updateInfo) {
	klog.V(4).Info("Received dev updates:", update)
	for devType, devices := range update.Added {
		var allocate allocateFunc
		var postAllocate postAllocateFunc
		var preStartContainer preStartContainerFunc
		var getPreferredAllocation getPreferredAllocationFunc

		if postAllocator, ok := m.devicePlugin.(PostAllocator); ok {
			postAllocate = postAllocator.PostAllocate
		}

		if containerPreStarter, ok := m.devicePlugin.(ContainerPreStarter); ok {
			preStartContainer = containerPreStarter.PreStartContainer
		}

		if preferredAllocator, ok := m.devicePlugin.(PreferredAllocator); ok {
			getPreferredAllocation = preferredAllocator.GetPreferredAllocation
		}

		if allocator, ok := m.devicePlugin.(Allocator); ok {
			allocate = allocator.Allocate
		}

		m.servers[devType] = m.createServer(devType, postAllocate, preStartContainer, getPreferredAllocation, allocate)
		go func(dt string) {
			err := m.servers[dt].Serve(m.namespace)
			if err != nil {
				klog.Errorf("Failed to serve %s/%s: %+v", m.namespace, dt, err)
				os.Exit(1)
			}
		}(devType)
		m.servers[devType].Update(devices)
	}
	for devType, devices := range update.Updated {
		m.servers[devType].Update(devices)
	}
	for devType := range update.Removed {
		if err := m.servers[devType].Stop(); err != nil {
			klog.Errorf("Unable to stop gRPC server for %q: %+v", devType, err)
		}
		delete(m.servers, devType)
	}
}
