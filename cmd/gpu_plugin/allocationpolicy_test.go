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
	"reflect"
	"sort"
	"testing"

	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func TestGetPreferredAllocation(t *testing.T) {
	rqt := &v1beta1.PreferredAllocationRequest{
		ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"card0-4", "card0-2", "card1-1", "card2-3", "card2-4", "card2-1", "card1-0", "card1-4", "card3-4", "card1-2", "card0-1", "card2-0", "card2-2", "card1-3", "card3-0", "card3-3", "card0-3", "card0-0", "card3-1", "card3-2"},
				AllocationSize:     4,
			},
		},
	}

	rqtNotEnough := &v1beta1.PreferredAllocationRequest{
		ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"card0-1", "card0-2", "card0-3", "card1-1"},
				AllocationSize:     3,
			},
		},
	}

	rqtErr := &v1beta1.PreferredAllocationRequest{
		ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"card0-4", "card1-1", "card2-3", "card2-4", "card2-1"},
				AllocationSize:     6,
			},
		},
	}

	plugin := newDevicePlugin("", "", cliOptions{sharedDevNum: 5, preferredAllocationPolicy: "none"})
	response, _ := plugin.GetPreferredAllocation(rqt)

	sort.Strings(response.ContainerResponses[0].DeviceIDs)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-4", "card1-1", "card2-3", "card3-4"}) {
		t.Error("Unexpected return value for none preferred allocation", response.ContainerResponses[0].DeviceIDs)
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, preferredAllocationPolicy: "balanced"})
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-0", "card1-0", "card2-0", "card3-0"}) {
		t.Error("Unexpected return value for balanced preferred allocation", response.ContainerResponses[0].DeviceIDs)
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, preferredAllocationPolicy: "packed"})
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-0", "card0-1", "card0-2", "card0-3"}) {
		t.Error("Unexpected return value for packed preferred allocation", response.ContainerResponses[0].DeviceIDs)
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, preferredAllocationPolicy: "none"})
	response, _ = plugin.GetPreferredAllocation(rqtErr)

	if response != nil {
		t.Error("Fail to handle the input error that req.AllocationSize is greater than len(req.AvailableDeviceIDs).")
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, preferredAllocationPolicy: "none"})
	response, _ = plugin.GetPreferredAllocation(rqtNotEnough)

	sort.Strings(response.ContainerResponses[0].DeviceIDs)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-1", "card0-2", "card1-1"}) {
		t.Error("Unexpected return value for none preferred allocation with too few separate devices",
			response.ContainerResponses[0].DeviceIDs)
	}
}
