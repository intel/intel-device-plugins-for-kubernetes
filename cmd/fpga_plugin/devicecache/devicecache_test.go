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

package devicecache

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

func createTestDirs(devfs, sysfs string, devfsDirs, sysfsDirs []string, sysfsFiles map[string][]byte) error {
	var err error

	for _, devfsdir := range devfsDirs {
		err = os.MkdirAll(path.Join(devfs, devfsdir), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create fake device directory: %v", err)
		}
	}
	for _, sysfsdir := range sysfsDirs {
		err = os.MkdirAll(path.Join(sysfs, sysfsdir), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create fake device directory: %v", err)
		}
	}
	for filename, body := range sysfsFiles {
		err = ioutil.WriteFile(path.Join(sysfs, filename), body, 0644)
		if err != nil {
			return fmt.Errorf("Failed to create fake vendor file: %v", err)
		}
	}

	return nil
}

func TestNewCache(t *testing.T) {
	tcases := []struct {
		mode        string
		expectedErr bool
	}{
		{
			mode: AfMode,
		},
		{
			mode: RegionMode,
		},
		{
			mode: RegionDevelMode,
		},
		{
			mode:        "unparsable",
			expectedErr: true,
		},
	}

	for _, tcase := range tcases {
		_, err := NewCache("", "", tcase.mode, nil)
		if tcase.expectedErr && err == nil {
			t.Error("No error generated when creating Cache with invalid parameters")
		}
	}
}

// getDevices returns static list of device structs for testing purposes
func getDevices() []device {
	return []device{
		{
			name: "intel-fpga-dev.0",
			regions: []region{
				{
					id:          "intel-fpga-fme.0",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/intel-fpga-fme.0",
					afus: []afu{
						{
							id:      "intel-fpga-port.0",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/intel-fpga-port.0",
						},
					},
				},
			},
		},
		{
			name: "intel-fpga-dev.1",
			regions: []region{
				{
					id:          "intel-fpga-fme.1",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/intel-fpga-fme.1",
					afus: []afu{
						{
							id:      "intel-fpga-port.1",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/intel-fpga-port.1",
						},
					},
				},
			},
		},
	}
}

func TestGetRegionDevelMap(t *testing.T) {
	expected := map[string]map[string]deviceplugin.DeviceInfo{
		RegionMode + "-ce48969398f05f33946d560708be108a": {
			"intel-fpga-fme.0": {
				State: pluginapi.Healthy,
				Nodes: []string{"/dev/intel-fpga-port.0", "/dev/intel-fpga-fme.0"},
			},
			"intel-fpga-fme.1": {
				State: pluginapi.Healthy,
				Nodes: []string{"/dev/intel-fpga-port.1", "/dev/intel-fpga-fme.1"},
			},
		},
	}

	result := getRegionDevelMap(getDevices())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetRegionMap(t *testing.T) {
	expected := map[string]map[string]deviceplugin.DeviceInfo{
		RegionMode + "-ce48969398f05f33946d560708be108a": {
			"intel-fpga-fme.0": {
				State: pluginapi.Healthy,
				Nodes: []string{"/dev/intel-fpga-port.0"},
			},
			"intel-fpga-fme.1": {
				State: pluginapi.Healthy,
				Nodes: []string{"/dev/intel-fpga-port.1"},
			},
		},
	}

	result := getRegionMap(getDevices())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetAfuMap(t *testing.T) {
	expected := map[string]map[string]deviceplugin.DeviceInfo{
		AfMode + "-d8424dc4a4a3c413f89e433683f9040b": {
			"intel-fpga-port.0": {
				State: pluginapi.Healthy,
				Nodes: []string{"/dev/intel-fpga-port.0"},
			},
			"intel-fpga-port.1": {
				State: pluginapi.Healthy,
				Nodes: []string{"/dev/intel-fpga-port.1"},
			},
		},
	}

	result := getAfuMap(getDevices())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func getDevMapClosure(oldmap map[string]map[string]deviceplugin.DeviceInfo, newmap map[string]map[string]deviceplugin.DeviceInfo) getDevMapFunc {
	var callnum int

	if oldmap == nil {
		oldmap = make(map[string]map[string]deviceplugin.DeviceInfo)
	}
	if newmap == nil {
		newmap = make(map[string]map[string]deviceplugin.DeviceInfo)
	}

	return func(devices []device) map[string]map[string]deviceplugin.DeviceInfo {
		defer func() { callnum = callnum + 1 }()

		if callnum%2 == 0 {
			return oldmap

		}
		return newmap
	}
}

func TestDetectUpdates(t *testing.T) {
	tcases := []struct {
		name            string
		expectedAdded   int
		expectedUpdated int
		expectedRemoved int
		oldmap          map[string]map[string]deviceplugin.DeviceInfo
		newmap          map[string]map[string]deviceplugin.DeviceInfo
	}{
		{
			name: "No devices found",
		},
		{
			name: "Added 1 new device type",
			newmap: map[string]map[string]deviceplugin.DeviceInfo{
				"fpgaID": {
					"intel-fpga-port.0": {
						State: pluginapi.Healthy,
						Nodes: []string{"/dev/intel-fpga-port.0"},
					},
				},
			},
			expectedAdded: 1,
		},
		{
			name: "Updated 1 new device type",
			oldmap: map[string]map[string]deviceplugin.DeviceInfo{
				"fpgaID": {
					"intel-fpga-port.0": {
						State: pluginapi.Healthy,
						Nodes: []string{"/dev/intel-fpga-port.0"},
					},
				},
			},
			newmap: map[string]map[string]deviceplugin.DeviceInfo{
				"fpgaID": {
					"intel-fpga-port.1": {
						State: pluginapi.Healthy,
						Nodes: []string{"/dev/intel-fpga-port.1"},
					},
				},
			},
			expectedUpdated: 1,
		},
		{
			name: "Removed 1 new device type",
			oldmap: map[string]map[string]deviceplugin.DeviceInfo{
				"fpgaID": {
					"intel-fpga-port.0": {
						State: pluginapi.Healthy,
						Nodes: []string{"/dev/intel-fpga-port.0"},
					},
				},
			},
			expectedRemoved: 1,
		},
	}

	for _, tcase := range tcases {
		ch := make(chan UpdateInfo, 1)
		cache, err := NewCache("", "", AfMode, ch)
		if err != nil {
			t.Fatal(err)
		}
		cache.getDevMap = getDevMapClosure(tcase.oldmap, tcase.newmap)

		cache.detectUpdates([]device{})

		var update UpdateInfo
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

func TestScanFPGAs(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/fpgaplugin-TestDiscoverFPGAs-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sys", "class", "fpga")
	devfs := path.Join(tmpdir, "dev")
	tcases := []struct {
		name            string
		devfsdirs       []string
		sysfsdirs       []string
		sysfsfiles      map[string][]byte
		errorContains   string
		expectedDevices []device
		mode            string
	}{
		{
			name:          "No sysfs folder given",
			mode:          AfMode,
			errorContains: "Can't read sysfs folder",
		},
		{
			name:          "FPGA device without FME and ports",
			mode:          AfMode,
			sysfsdirs:     []string{"intel-fpga-dev.0", "incorrect-file-name"},
			errorContains: "No regions found",
		},
		{
			name:          "AFU without ID",
			mode:          AfMode,
			sysfsdirs:     []string{"intel-fpga-dev.0/intel-fpga-port.0"},
			errorContains: "afu_id: no such file or directory",
		},
		{
			name:      "No device node for detected AFU",
			mode:      AfMode,
			sysfsdirs: []string{"intel-fpga-dev.0/intel-fpga-port.0"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "/dev/intel-fpga-port.0: no such file or directory",
		},
		{
			name:      "AFU without corresponding FME",
			mode:      AfMode,
			sysfsdirs: []string{"intel-fpga-dev.0/intel-fpga-port.0"},
			devfsdirs: []string{"intel-fpga-port.0"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "No regions found",
		},
		{
			name: "More than one FME per FPGA device",
			mode: AfMode,
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"intel-fpga-dev.0/intel-fpga-fme.1/pr",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.0/intel-fpga-fme.1/pr/interface_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			devfsdirs: []string{
				"intel-fpga-fme.0",
				"intel-fpga-fme.1",
			},
			errorContains: "Detected more than one FPGA region",
		},
		{
			name:          "FME without interface ID",
			mode:          AfMode,
			sysfsdirs:     []string{"intel-fpga-dev.0/intel-fpga-fme.0"},
			errorContains: "interface_id: no such file or directory",
		},
		{
			name:      "No device node for detected region",
			mode:      AfMode,
			sysfsdirs: []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "/dev/intel-fpga-fme.0: no such file or directory",
		},
		{
			name: "No errors expected in af mode",
			mode: AfMode,
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
				"intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"intel-fpga-dev.1/intel-fpga-port.1",
				"intel-fpga-dev.1/intel-fpga-fme.1/pr",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id":         []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.1/intel-fpga-port.1/afu_id":         []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a\n"),
				"intel-fpga-dev.1/intel-fpga-fme.1/pr/interface_id": []byte("ce48969398f05f33946d560708be108a\n"),
			},
			devfsdirs: []string{
				"intel-fpga-port.0", "intel-fpga-fme.0",
				"intel-fpga-port.1", "intel-fpga-fme.1",
			},
			expectedDevices: []device{
				{
					name: "intel-fpga-dev.0",
					regions: []region{
						{
							id:          "intel-fpga-fme.0",
							interfaceID: "ce48969398f05f33946d560708be108a",
							devNode:     path.Join(devfs, "intel-fpga-fme.0"),
							afus: []afu{
								{
									id:      "intel-fpga-port.0",
									afuID:   "d8424dc4a4a3c413f89e433683f9040b",
									devNode: path.Join(devfs, "intel-fpga-port.0"),
								},
							},
						},
					},
				},
				{
					name: "intel-fpga-dev.1",
					regions: []region{
						{
							id:          "intel-fpga-fme.1",
							interfaceID: "ce48969398f05f33946d560708be108a",
							devNode:     path.Join(devfs, "intel-fpga-fme.1"),
							afus: []afu{
								{
									id:      "intel-fpga-port.1",
									afuID:   "d8424dc4a4a3c413f89e433683f9040b",
									devNode: path.Join(devfs, "intel-fpga-port.1"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "No errors expected in region mode",
			mode: RegionMode,
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
				"intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"intel-fpga-dev.1/intel-fpga-port.1",
				"intel-fpga-dev.1/intel-fpga-fme.1/pr",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a\n"),
				"intel-fpga-dev.1/intel-fpga-fme.1/pr/interface_id": []byte("ce48969398f05f33946d560708be108a\n"),
			},
			devfsdirs: []string{
				"intel-fpga-port.0", "intel-fpga-fme.0",
				"intel-fpga-port.1", "intel-fpga-fme.1",
			},
			expectedDevices: []device{
				{
					name: "intel-fpga-dev.0",
					regions: []region{
						{
							id:          "intel-fpga-fme.0",
							interfaceID: "ce48969398f05f33946d560708be108a",
							devNode:     path.Join(devfs, "intel-fpga-fme.0"),
							afus: []afu{
								{
									id:      "intel-fpga-port.0",
									afuID:   "unused_afu_id",
									devNode: path.Join(devfs, "intel-fpga-port.0"),
								},
							},
						},
					},
				},
				{
					name: "intel-fpga-dev.1",
					regions: []region{
						{
							id:          "intel-fpga-fme.1",
							interfaceID: "ce48969398f05f33946d560708be108a",
							devNode:     path.Join(devfs, "intel-fpga-fme.1"),
							afus: []afu{
								{
									id:      "intel-fpga-port.1",
									afuID:   "unused_afu_id",
									devNode: path.Join(devfs, "intel-fpga-port.1"),
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tcase := range tcases {
		err := createTestDirs(devfs, sysfs, tcase.devfsdirs, tcase.sysfsdirs, tcase.sysfsfiles)
		if err != nil {
			t.Fatal(err)
		}

		cache, err := NewCache(sysfs, devfs, tcase.mode, nil)
		if err != nil {
			t.Fatal(err)
		}
		cache.getDevMap = func(devices []device) map[string]map[string]deviceplugin.DeviceInfo {
			return make(map[string]map[string]deviceplugin.DeviceInfo)
		}

		err = cache.scanFPGAs()
		if tcase.errorContains != "" {
			if err == nil || !strings.Contains(err.Error(), tcase.errorContains) {
				t.Errorf("Test case '%s': expected error '%s', but got '%v'", tcase.name, tcase.errorContains, err)
			}
		} else if err != nil {
			t.Errorf("Test case '%s': expected no error, but got '%v'", tcase.name, err)
		}
		if tcase.expectedDevices != nil && !reflect.DeepEqual(tcase.expectedDevices, cache.devices) {
			t.Errorf("Test case '%s': expected devices '%v', but got '%v'", tcase.name, tcase.expectedDevices, cache.devices)
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRun(t *testing.T) {
	cache := Cache{
		sysfsDir: "/dev/null",
	}
	err := cache.Run()
	if err == nil {
		t.Error("Expected error, but got nil")
	}
}
