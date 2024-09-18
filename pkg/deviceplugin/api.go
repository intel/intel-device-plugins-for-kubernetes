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
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

// DeviceInfo contains information about device maintained by Device Plugin.
type DeviceInfo struct {
	mounts      []pluginapi.Mount
	envs        map[string]string
	annotations map[string]string
	topology    *pluginapi.TopologyInfo
	// https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/4009-add-cdi-devices-to-device-plugin-api
	cdiSpec *cdispec.Spec
	state   string
	nodes   []pluginapi.DeviceSpec
}

// UseDefaultMethodError allows the plugin to request running the default
// logic even while implementing an optional interface. This is currently
// supported only with the Allocator interface.
type UseDefaultMethodError struct{}

func (e *UseDefaultMethodError) Error() string {
	return "use default method"
}

func init() {
	klog.InitFlags(nil)
}

// NewDeviceInfo makes DeviceInfo struct and adds topology information to it.
func NewDeviceInfo(state string, nodes []pluginapi.DeviceSpec, mounts []pluginapi.Mount, envs, annotations map[string]string, cdiSpec *cdispec.Spec, devPaths ...string) DeviceInfo {
	deviceInfo := DeviceInfo{
		state:       state,
		nodes:       nodes,
		mounts:      mounts,
		envs:        envs,
		annotations: annotations,
		cdiSpec:     cdiSpec,
	}

	var devPathsComputed []string

	// If devPaths are provided, use them; otherwise, generate from nodes
	if len(devPaths) > 0 && devPaths[0] != "" {
		devPathsComputed = devPaths
	} else {
		devPathsComputed = []string{}
		for _, node := range nodes {
			devPathsComputed = append(devPathsComputed, node.HostPath)
		}
	}

	// Get topology information based on devPaths
	topologyInfo, err := topology.GetTopologyInfo(devPathsComputed)
	if err == nil {
		deviceInfo.topology = topologyInfo
	} else {
		klog.Warningf("GetTopologyInfo: %v", err)
	}

	return deviceInfo
}

// NewDeviceInfoWithTopologyHints makes DeviceInfo struct with topology information provided to it.
func NewDeviceInfoWithTopologyHints(state string, nodes []pluginapi.DeviceSpec, mounts []pluginapi.Mount, envs map[string]string,
	annotations map[string]string, topology *pluginapi.TopologyInfo, cdiSpec *cdispec.Spec) DeviceInfo {
	return DeviceInfo{
		state:       state,
		nodes:       nodes,
		mounts:      mounts,
		envs:        envs,
		annotations: annotations,
		topology:    topology,
		cdiSpec:     cdiSpec,
	}
}

// DeviceTree contains a tree-like structure of device type -> device ID -> device info.
type DeviceTree map[string]map[string]DeviceInfo

// NewDeviceTree creates an instance of DeviceTree.
func NewDeviceTree() DeviceTree {
	return make(map[string]map[string]DeviceInfo)
}

// AddDevice adds device info to DeviceTree.
func (tree DeviceTree) AddDevice(devType, id string, info DeviceInfo) {
	if _, present := tree[devType]; !present {
		tree[devType] = make(map[string]DeviceInfo)
	}

	if info.cdiSpec != nil {
		devLength := len(info.cdiSpec.Devices)
		if devLength == 0 {
			klog.Warning("No CDI devices defined in spec, removing spec")

			info.cdiSpec = nil
		} else if devLength > 1 {
			klog.Warning("Including more than one CDI device per spec is not supported, using first")

			info.cdiSpec.Devices = info.cdiSpec.Devices[:1]
		}
	}

	tree[devType][id] = info
}

// DeviceTypeCount returns number of device of given type.
func (tree DeviceTree) DeviceTypeCount(devType string) int {
	return len(tree[devType])
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

// Allocator is an optional interface implemented by device plugins.
type Allocator interface {
	// Allocate allows the plugin to replace the server Allocate(). Plugin can return
	// UseDefaultAllocateMethod if the default server allocation is anyhow preferred
	// for the particular allocation request.
	Allocate(*pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error)
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
