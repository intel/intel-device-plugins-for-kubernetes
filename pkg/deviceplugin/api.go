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

// Package deviceplugin provides API for reporting available devices to kubelet.
package deviceplugin

import (
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/topology"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// DeviceInfo contains information about device maintained by Device Plugin.
type DeviceInfo struct {
	state    string
	nodes    []pluginapi.DeviceSpec
	mounts   []pluginapi.Mount
	envs     map[string]string
	topology *pluginapi.TopologyInfo
}

func init() {
	klog.InitFlags(nil)
}

// NewDeviceInfo makes DeviceInfo struct and adds topology information to it.
func NewDeviceInfo(state string, nodes []pluginapi.DeviceSpec, mounts []pluginapi.Mount, envs map[string]string) DeviceInfo {
	deviceInfo := DeviceInfo{
		state:  state,
		nodes:  nodes,
		mounts: mounts,
		envs:   envs,
	}
	devPaths := []string{}
	for _, node := range nodes {
		devPaths = append(devPaths, node.HostPath)
	}

	topologyInfo, err := topology.GetTopologyInfo(devPaths)
	if err == nil {
		deviceInfo.topology = topologyInfo
	} else {
		klog.Warningf("GetTopologyInfo: %v", err)
	}

	return deviceInfo
}

// DeviceTree contains a tree-like structure of device type -> device ID -> device info.
type DeviceTree interface {
	// AddDevice adds device info to DeviceTree.
	AddDevice(devType, id string, info DeviceInfo)
	// AsMap returns a map of maps representing the device tree.
	// The first dimension of the map is a device type.
	// The second dimension is a device ID.
	// The value of the secondary maps are instances of DeviceInfo.
	AsMap() map[string]map[string]DeviceInfo
}

// deviceTree implements DeviceTree interface.
type deviceTree map[string]map[string]DeviceInfo

// NewDeviceTree creates an instance of DeviceTree.
func NewDeviceTree() DeviceTree {
	return make(deviceTree)
}

// AddDevice adds device info to DeviceTree.
func (tree deviceTree) AddDevice(devType, id string, info DeviceInfo) {
	if _, present := tree[devType]; !present {
		tree[devType] = make(map[string]DeviceInfo)
	}
	tree[devType][id] = info
}

func (tree deviceTree) AsMap() map[string]map[string]DeviceInfo {
	return tree
}

// Notifier receives updates from Scanner, detects changes and sends the
// detected changes to a channel given by the creator of a Notifier object.
type Notifier interface {
	// Notify notifies manager with a device tree constructed by device
	// plugin during scanning process.
	Notify(DeviceTree)
}

// Scanner serves as an interface between Manager and a device plugin.
type Scanner interface {
	// Scan scans the host for devices and sends all found devices to
	// a Notifier instance. It's called only once for every device plugin by
	// Manager in a goroutine and operates in an infinite loop.
	Scan(Notifier) error
}

// PostAllocator is an optional interface implemented by device plugins.
type PostAllocator interface {
	// PostAllocate modifies responses returned by Allocate() by e.g.
	// adding annotations consumed by CRI hooks to the responses.
	PostAllocate(*pluginapi.AllocateResponse) error
}

// PreferredAllocator is an optional interface implemented by device plugins.
type PreferredAllocator interface {
	// GetPreferredAllocation defines the list of devices preferred for allocating next.
	GetPreferredAllocation(*pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error)
}

// ContainerPreStarter is an optional interface implemented by device plugins.
type ContainerPreStarter interface {
	// PreStartContainer  defines device initialization function before container is started.
	// It might include operations like card reset.
	PreStartContainer(*pluginapi.PreStartContainerRequest) error
}
