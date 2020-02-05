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
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/intel/cri-resource-manager/pkg/topology"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// DeviceInfo contains information about device maintained by Device Plugin
type DeviceInfo struct {
	state    string
	nodes    []pluginapi.DeviceSpec
	mounts   []pluginapi.Mount
	envs     map[string]string
	topology *pluginapi.TopologyInfo
}

// getTopologyInfo returns topology information for the list of device nodes
func getTopologyInfo(devs []string) (*pluginapi.TopologyInfo, error) {
	var result pluginapi.TopologyInfo
	nodeIDs := map[int64]struct{}{}
	for _, dev := range devs {
		sysfsDevice, err := topology.FindSysFsDevice(dev)
		if err != nil {
			return nil, err
		}

		if sysfsDevice == "" {
			return nil, errors.Errorf("device %s doesn't exist", dev)
		}

		hints, err := topology.NewTopologyHints(sysfsDevice)
		if err != nil {
			return nil, err
		}

		for _, hint := range hints {
			for _, nNode := range strings.Split(hint.NUMAs, ",") {
				nNodeID, err := strconv.ParseInt(strings.TrimSpace(nNode), 10, 64)
				if err != nil {
					return nil, errors.Wrapf(err, "unable to convert numa node %s into int64", nNode)
				}
				if nNodeID < 0 {
					return nil, errors.Wrapf(err, "numa node is negative: %d", nNodeID)
				}
				if _, ok := nodeIDs[nNodeID]; !ok {
					result.Nodes = append(result.Nodes, &pluginapi.NUMANode{ID: nNodeID})
					nodeIDs[nNodeID] = struct{}{}
				}
			}
		}
	}
	sort.Slice(result.Nodes, func(i, j int) bool { return result.Nodes[i].ID < result.Nodes[j].ID })
	return &result, nil
}

// NewDeviceInfo makes DeviceInfo struct and adds topology information to it
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

	topologyInfo, err := getTopologyInfo(devPaths)
	if err == nil {
		deviceInfo.topology = topologyInfo
	}

	return deviceInfo
}

// DeviceTree contains a tree-like structure of device type -> device ID -> device info.
type DeviceTree map[string]map[string]DeviceInfo

// NewDeviceTree creates an instance of DeviceTree
func NewDeviceTree() DeviceTree {
	return make(map[string]map[string]DeviceInfo)
}

// AddDevice adds device info to DeviceTree.
func (tree DeviceTree) AddDevice(devType, id string, info DeviceInfo) {
	if _, present := tree[devType]; !present {
		tree[devType] = make(map[string]DeviceInfo)
	}
	tree[devType][id] = info
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
