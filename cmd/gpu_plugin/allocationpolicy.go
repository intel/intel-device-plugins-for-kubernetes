// Copyright 2017-2026 Intel Corporation. All Rights Reserved.
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

package main

import (
	"sort"
	"strings"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type preferredAllocationPolicyFunc func(*pluginapi.ContainerPreferredAllocationRequest) []string

// nonePolicy is used for allocating GPU devices randomly, while trying
// to select as many individual GPU devices as requested.
func nonePolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	klog.V(2).Info("Select nonePolicy for GPU device allocation")

	devices := make(map[string]bool)
	selected := make(map[string]bool)
	neededCount := req.AllocationSize

	// When shared-dev-num is greater than 1, try to find as
	// many independent GPUs as possible, to satisfy the request.

	for _, deviceID := range req.AvailableDeviceIDs {
		device := strings.Split(deviceID, "-")[0]

		if _, found := devices[device]; !found {
			devices[device] = true
			selected[deviceID] = true
			neededCount--

			if neededCount == 0 {
				break
			}
		}
	}

	// If there were not enough independent GPUs, use remaining untaken deviceIDs.

	if neededCount > 0 {
		for _, deviceID := range req.AvailableDeviceIDs {
			if _, found := selected[deviceID]; !found {
				selected[deviceID] = true
				neededCount--

				if neededCount == 0 {
					break
				}
			}
		}
	}

	// Convert selected map into an array

	deviceIDs := []string{}

	for deviceID := range selected {
		deviceIDs = append(deviceIDs, deviceID)
	}

	klog.V(2).Infof("Allocate deviceIds: %q", deviceIDs)

	return deviceIDs
}

// balancedPolicy is used for allocating GPU devices in balance.
func balancedPolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	klog.V(2).Info("Select balancedPolicy for GPU device allocation")

	// Save the shared-devices list of each physical GPU.
	Card := make(map[string][]string)
	// Save the available shared-nums of each physical GPU.
	Count := make(map[string]int)

	for _, deviceID := range req.AvailableDeviceIDs {
		device := strings.Split(deviceID, "-")
		Card[device[0]] = append(Card[device[0]], deviceID)
		Count[device[0]]++
	}

	// Save the physical GPUs in order.
	Index := make([]string, 0)
	for key := range Count {
		Index = append(Index, key)
		sort.Strings(Card[key])
	}

	sort.Strings(Index)

	need := req.AllocationSize

	var deviceIds []string

	// We choose one device ID from the GPU card that has most shared gpu IDs each time.
	for {
		var (
			allocateCard string
			max          int
		)

		for _, key := range Index {
			if Count[key] > max {
				max = Count[key]
				allocateCard = key
			}
		}

		deviceIds = append(deviceIds, Card[allocateCard][0])
		need--

		if need == 0 {
			break
		}

		// Update Maps
		Card[allocateCard] = Card[allocateCard][1:]
		Count[allocateCard]--
	}

	klog.V(2).Infof("Allocate deviceIds: %q", deviceIds)

	return deviceIds
}

// packedPolicy is used for allocating GPU devices one by one.
func packedPolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	klog.V(2).Info("Select packedPolicy for GPU device allocation")

	deviceIds := req.AvailableDeviceIDs
	sort.Strings(deviceIds)
	deviceIds = deviceIds[:req.AllocationSize]

	klog.V(2).Infof("Allocate deviceIds: %q", deviceIds)

	return deviceIds
}
