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

package main

import (
	"fmt"
	"testing"

	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/devicecache"
	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

// Minimal implementation of pluginapi.DevicePlugin_ListAndWatchServer
type listAndWatchServerStub struct {
	testDM      *deviceManager
	generateErr bool
	cdata       chan []*pluginapi.Device
}

func (s *listAndWatchServerStub) Send(resp *pluginapi.ListAndWatchResponse) error {
	if s.generateErr {
		fmt.Println("listAndWatchServerStub::Send returns error")
		return fmt.Errorf("Fake error")
	}

	fmt.Println("listAndWatchServerStub::Send", resp.Devices)
	s.cdata <- resp.Devices
	return nil
}

func (s *listAndWatchServerStub) Context() context.Context {
	return nil
}

func (s *listAndWatchServerStub) RecvMsg(m interface{}) error {
	return nil
}

func (s *listAndWatchServerStub) SendMsg(m interface{}) error {
	return nil
}

func (s *listAndWatchServerStub) SendHeader(m metadata.MD) error {
	return nil
}

func (s *listAndWatchServerStub) SetHeader(m metadata.MD) error {
	return nil
}

func (s *listAndWatchServerStub) SetTrailer(m metadata.MD) {
}

func TestGetDevicePluginOptions(t *testing.T) {
	dm := &deviceManager{}
	dm.GetDevicePluginOptions(nil, nil)
}

func TestPreStartContainer(t *testing.T) {
	dm := &deviceManager{}
	dm.PreStartContainer(nil, nil)
}

func TestListAndWatch(t *testing.T) {
	tcases := []struct {
		name        string
		updates     []map[string]deviceplugin.DeviceInfo
		expectedErr bool
	}{
		{
			name: "No updates and close",
		},
		{
			name: "Send 1 update",
			updates: []map[string]deviceplugin.DeviceInfo{
				{
					"fake_id": {
						State: pluginapi.Healthy,
						Nodes: []string{"/dev/intel-fpga-port.0"},
					},
				},
			},
		},
		{
			name: "Send 1 update, but expect streaming error",
			updates: []map[string]deviceplugin.DeviceInfo{
				{
					"fake_id": {
						State: pluginapi.Healthy,
						Nodes: []string{"/dev/intel-fpga-port.0"},
					},
				},
			},
			expectedErr: true,
		},
	}

	for _, tt := range tcases {
		devCh := make(chan map[string]deviceplugin.DeviceInfo, len(tt.updates))
		testDM := newDeviceManager("intel.com/fpgatest-fpgaID", "fpgaID", devicecache.AfMode, devCh)

		server := &listAndWatchServerStub{
			testDM:      testDM,
			generateErr: tt.expectedErr,
			cdata:       make(chan []*pluginapi.Device, len(tt.updates)),
		}

		// push device infos to DM's channel
		for _, update := range tt.updates {
			devCh <- update
		}
		close(devCh)

		err := testDM.ListAndWatch(&pluginapi.Empty{}, server)
		if err != nil && !tt.expectedErr {
			t.Errorf("Test case '%s': got unexpected error %v", tt.name, err)
		}
		if err == nil && tt.expectedErr {
			t.Errorf("Test case '%s': expected an error, but got nothing", tt.name)
		}
	}
}

func TestAllocate(t *testing.T) {
	testDM := newDeviceManager("", "", devicecache.RegionMode, nil)
	if testDM == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	rqt := &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{
				DevicesIDs: []string{"dev1"},
			},
		},
	}

	testDM.devices["dev1"] = deviceplugin.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []string{"/dev/dev1"},
	}
	resp, err := testDM.Allocate(nil, rqt)
	if err != nil {
		t.Fatalf("Failed to allocate healthy device: %v", err)
	}

	if len(resp.ContainerResponses[0].Devices) != 1 {
		t.Fatal("Allocated wrong number of devices")
	}

	if len(resp.ContainerResponses[0].Annotations) != 1 {
		t.Fatal("Set wrong number of annotations")
	}

	annotation, ok := resp.ContainerResponses[0].Annotations[annotationName]
	if ok == false {
		t.Fatalf("%s annotation is not set", annotationName)
	}

	expectedAnnotationValue := fmt.Sprintf("%s-%s", resourceNamePrefix, devicecache.RegionMode)
	if annotation != expectedAnnotationValue {
		t.Fatalf("Set wrong %s annotation value %s, should be %s", resourceNamePrefix, annotation, expectedAnnotationValue)
	}
}

func startDeviceManagerStub(dm *deviceManager, pluginPrefix string) {
}

func TestHandleUpdate(t *testing.T) {
	tcases := []struct {
		name        string
		dms         map[string]*deviceManager
		updateInfo  devicecache.UpdateInfo
		expectedDMs int
	}{
		{
			name:        "Empty update",
			updateInfo:  devicecache.UpdateInfo{},
			expectedDMs: 0,
		},
		{
			name: "Add device manager",
			updateInfo: devicecache.UpdateInfo{
				Added: map[string]map[string]deviceplugin.DeviceInfo{
					"ce48969398f05f33946d560708be108a": {
						"intel-fpga-fme.0": {
							State: pluginapi.Healthy,
							Nodes: []string{"/dev/intel-fpga-port.0", "/dev/intel-fpga-fme.0"},
						},
						"intel-fpga-fme.1": {
							State: pluginapi.Healthy,
							Nodes: []string{"/dev/intel-fpga-port.1", "/dev/intel-fpga-fme.1"},
						},
					},
				},
			},
			expectedDMs: 1,
		},
		{
			name: "Update existing device manager",
			dms: map[string]*deviceManager{
				"ce48969398f05f33946d560708be108a": {
					ch: make(chan map[string]deviceplugin.DeviceInfo, 1),
				},
			},
			updateInfo: devicecache.UpdateInfo{
				Updated: map[string]map[string]deviceplugin.DeviceInfo{
					"ce48969398f05f33946d560708be108a": {
						"intel-fpga-fme.1": {
							State: pluginapi.Healthy,
							Nodes: []string{"/dev/intel-fpga-port.1", "/dev/intel-fpga-fme.1"},
						},
					},
				},
			},
			expectedDMs: 1,
		},
		{
			name: "Remove device manager",
			dms: map[string]*deviceManager{
				"ce48969398f05f33946d560708be108a": {
					ch: make(chan map[string]deviceplugin.DeviceInfo, 1),
				},
			},
			updateInfo: devicecache.UpdateInfo{
				Removed: map[string]map[string]deviceplugin.DeviceInfo{
					"ce48969398f05f33946d560708be108a": {},
				},
			},
			expectedDMs: 0,
		},
	}

	for _, tt := range tcases {
		if tt.dms == nil {
			tt.dms = make(map[string]*deviceManager)
		}
		handleUpdate(tt.dms, tt.updateInfo, startDeviceManagerStub, devicecache.AfMode)
		if len(tt.dms) != tt.expectedDMs {
			t.Errorf("Test case '%s': expected %d runnig device managers, but got %d",
				tt.name, tt.expectedDMs, len(tt.dms))
		}
	}
}
