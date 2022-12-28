// Copyright 2017-2021 Intel Corporation. All Rights Reserved.
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
	"flag"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/pkg/errors"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/rm"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

// mockNotifier implements Notifier interface.
type mockNotifier struct {
	scanDone     chan bool
	devCount     int
	monitorCount int
}

// Notify stops plugin Scan.
func (n *mockNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.monitorCount = len(newDeviceTree[monitorType])
	n.devCount = len(newDeviceTree[deviceType])
	n.scanDone <- true
}

type mockResourceManager struct{}

func (m *mockResourceManager) CreateFractionalResourceResponse(*v1beta1.AllocateRequest) (*v1beta1.AllocateResponse, error) {
	return &v1beta1.AllocateResponse{}, &dpapi.UseDefaultMethodError{}
}
func (m *mockResourceManager) SetDevInfos(rm.DeviceInfoMap) {}

func (m *mockResourceManager) GetPreferredFractionalAllocation(*v1beta1.PreferredAllocationRequest) (*v1beta1.PreferredAllocationResponse, error) {
	return &v1beta1.PreferredAllocationResponse{}, &dpapi.UseDefaultMethodError{}
}

func createTestFiles(root string, devfsdirs, sysfsdirs []string, sysfsfiles map[string][]byte) (string, string, error) {
	sysfs := path.Join(root, "sys")
	devfs := path.Join(root, "dev")

	for _, devfsdir := range devfsdirs {
		if err := os.MkdirAll(path.Join(devfs, devfsdir), 0750); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	for _, sysfsdir := range sysfsdirs {
		if err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0750); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	for filename, body := range sysfsfiles {
		if err := os.WriteFile(path.Join(sysfs, filename), body, 0600); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	return sysfs, devfs, nil
}

func TestNewDevicePlugin(t *testing.T) {
	if newDevicePlugin("", "", cliOptions{sharedDevNum: 2, resourceManagement: false}) == nil {
		t.Error("Failed to create plugin")
	}

	if newDevicePlugin("", "", cliOptions{sharedDevNum: 2, resourceManagement: true}) != nil {
		t.Error("Unexpectedly managed to create resource management enabled plugin inside unit tests")
	}
}

func TestGetPreferredAllocation(t *testing.T) {
	rqt := &v1beta1.PreferredAllocationRequest{
		ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"card0-4", "card1-1", "card2-3", "card2-4", "card2-1", "card1-0", "card1-4", "card3-4", "card1-2", "card0-1", "card2-0", "card2-2", "card1-3", "card0-2", "card3-0", "card3-3", "card0-3", "card0-0", "card3-1", "card3-2"},
				AllocationSize:     4,
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

	plugin := newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "none"})
	response, _ := plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-4", "card1-1", "card2-3", "card2-4"}) {
		t.Error("Unexpected return value for none preferred allocation")
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "balanced"})
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-0", "card1-0", "card2-0", "card3-0"}) {
		t.Error("Unexpected return value for balanced preferred allocation")
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "packed"})
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-0", "card0-1", "card0-2", "card0-3"}) {
		t.Error("Unexpected return value for packed preferred allocation")
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "none"})
	response, _ = plugin.GetPreferredAllocation(rqtErr)

	if response != nil {
		t.Error("Fail to handle the input error that req.AllocationSize is greater than len(req.AvailableDeviceIDs).")
	}
}

func TestAllocate(t *testing.T) {
	plugin := newDevicePlugin("", "", cliOptions{sharedDevNum: 2, resourceManagement: false})

	_, err := plugin.Allocate(&v1beta1.AllocateRequest{})
	if _, ok := err.(*dpapi.UseDefaultMethodError); !ok {
		t.Errorf("Unexpected return value: %+v", err)
	}

	// mock the rm
	plugin.resMan = &mockResourceManager{}

	_, err = plugin.Allocate(&v1beta1.AllocateRequest{})
	if _, ok := err.(*dpapi.UseDefaultMethodError); !ok {
		t.Errorf("Unexpected return value: %+v", err)
	}
}

func TestScan(t *testing.T) {
	tcases := []struct {
		name string
		// test-case environment
		sysfsdirs  []string
		sysfsfiles map[string][]byte
		devfsdirs  []string
		// how plugin should interpret it
		options cliOptions
		// what the result should be
		expectedDevs     int
		expectedMonitors int
	}{
		{
			name: "no sysfs mounted",
		},
		{
			name:      "no device installed",
			sysfsdirs: []string{"card0"},
		},
		{
			name:      "missing dev node",
			sysfsdirs: []string{"card0/device"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
		},
		{
			name:      "one device",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
			devfsdirs:    []string{"card0"},
			expectedDevs: 1,
		},
		{
			name:      "sriov-1-pf-no-vfs + monitoring",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":       []byte("0x8086"),
				"card0/device/sriov_numvfs": []byte("0"),
			},
			devfsdirs:        []string{"card0"},
			options:          cliOptions{enableMonitoring: true},
			expectedDevs:     1,
			expectedMonitors: 1,
		},
		{
			name: "two sysfs records but one dev node",
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			devfsdirs:    []string{"card0"},
			expectedDevs: 1,
		},
		{
			name: "sriov-1-pf-and-2-vfs",
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card2/device/drm/card2",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":       []byte("0x8086"),
				"card0/device/sriov_numvfs": []byte("2"),
				"card1/device/vendor":       []byte("0x8086"),
				"card2/device/vendor":       []byte("0x8086"),
			},
			devfsdirs:    []string{"card0", "card1", "card2"},
			expectedDevs: 2,
		},
		{
			name: "two devices with 13 shares + monitoring",
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card0/gt/gt0/error_counter",
				"card1/gt/gt0/error_counter",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
				"card0/gt/gt0/error_counter/fatal_guc": []byte("0"),
				"card1/gt/gt0/error_counter/sgunit_fatal": []byte("0"),
			},
			devfsdirs:        []string{"card0", "card1"},
			options:          cliOptions{sharedDevNum: 13, enableMonitoring: true},
			expectedDevs:     26,
			expectedMonitors: 1,
		},
		{
			name: "two devices with with fatal errors + monitoring",
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card0/gt/gt0/error_counter",
				"card1/gt/gt0/error_counter",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
				"card0/gt/gt0/error_counter/fatal_guc": []byte("1"),
				"card1/gt/gt0/error_counter/sgunit_fatal": []byte("1"),
			},
			devfsdirs:        []string{"card0", "card1"},
			options:          cliOptions{sharedDevNum: 13, enableMonitoring: true},
			expectedDevs:     0,
			expectedMonitors: 1,
		},
		{
			name:      "wrong vendor",
			sysfsdirs: []string{"card0/device/drm/card0"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0xbeef"),
			},
			devfsdirs: []string{"card0"},
		},
		{
			name:      "wrong vendor with 13 shares + monitoring",
			sysfsdirs: []string{"card0/device/drm/card0"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0xbeef"),
			},
			devfsdirs: []string{"card0"},
			options:   cliOptions{sharedDevNum: 13, enableMonitoring: true},
		},
		{
			name:      "no sysfs records",
			sysfsdirs: []string{"non_gpu_card"},
		},
	}

	for _, tc := range tcases {
		if tc.options.sharedDevNum == 0 {
			tc.options.sharedDevNum = 1
		}

		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp("", "test_new_device_plugin")
			if err != nil {
				t.Fatalf("can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			sysfs, devfs, err := createTestFiles(root, tc.devfsdirs, tc.sysfsdirs, tc.sysfsfiles)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			plugin := newDevicePlugin(sysfs, devfs, tc.options)

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

			plugin.resMan = &mockResourceManager{}

			err = plugin.Scan(notifier)
			// Scans in GPU plugin never fail
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if tc.expectedDevs != notifier.devCount {
				t.Errorf("Expected %d, discovered %d devices",
					tc.expectedDevs, notifier.devCount)
			}
			if tc.expectedMonitors != notifier.monitorCount {
				t.Errorf("Expected %d, discovered %d monitors",
					tc.expectedMonitors, notifier.monitorCount)
			}
		})
	}
}
