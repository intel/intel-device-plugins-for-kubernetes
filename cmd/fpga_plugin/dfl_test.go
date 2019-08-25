// Copyright 2019 Intel Corporation. All Rights Reserved.
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
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

func TestNewDevicePluginDFL(t *testing.T) {
	tcases := []struct {
		mode        string
		expectedErr bool
	}{
		{
			mode: afMode,
		},
		{
			mode: regionMode,
		},
		{
			mode: regionDevelMode,
		},
		{
			mode:        "unparsable",
			expectedErr: true,
		},
	}

	for _, tcase := range tcases {
		_, err := newDevicePluginDFL("", "", tcase.mode)
		if tcase.expectedErr && err == nil {
			t.Error("No error generated when creating Cache with invalid parameters")
		}
	}
}

// getDevices returns static list of device structs for testing purposes
func getDevicesDFL() []device {
	return []device{
		{
			name: "region1",
			regions: []region{
				{
					id:          "region1",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/dfl-fme.0",
					afus: []afu{
						{
							id:      "dfl-port.0",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/dfl-port.0",
						},
					},
				},
			},
		},
		{
			name: "region2",
			regions: []region{
				{
					id:          "region2",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/dfl-fme.1",
					afus: []afu{
						{
							id:      "dfl-port.1",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/dfl-port.1",
						},
						{
							id:      "dfl-port.2",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/dfl-port.2",
						},
					},
				},
			},
		},
		{
			name: "region3",
			regions: []region{
				{
					id:          "region3",
					interfaceID: unhealthyInterfaceID,
					devNode:     "/dev/dfl-fme.2",
					afus: []afu{
						{
							id:      "dfl-port.3",
							afuID:   unhealthyAfuID,
							devNode: "/dev/dfl-port.3",
						},
						{
							id:      "dfl-port.4",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/dfl-port.4",
						},
					},
				},
			},
		},
	}
}

func TestGetRegionDevelTreeDFL(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.0",
				ContainerPath: "/dev/dfl-port.0",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/dfl-fme.0",
				ContainerPath: "/dev/dfl-fme.0",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region2", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.1",
				ContainerPath: "/dev/dfl-port.1",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/dfl-port.2",
				ContainerPath: "/dev/dfl-port.2",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/dfl-fme.1",
				ContainerPath: "/dev/dfl-fme.1",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "region3", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.3",
				ContainerPath: "/dev/dfl-port.3",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/dfl-port.4",
				ContainerPath: "/dev/dfl-port.4",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/dfl-fme.2",
				ContainerPath: "/dev/dfl-fme.2",
				Permissions:   "rw",
			},
		},
	})

	result := getRegionDevelTree(getDevicesDFL())
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Got unexpected result: %v, expected: %v", result, expected)
	}
}

func TestGetRegionTreeDFL(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.0",
				ContainerPath: "/dev/dfl-port.0",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region2", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.1",
				ContainerPath: "/dev/dfl-port.1",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/dfl-port.2",
				ContainerPath: "/dev/dfl-port.2",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "region3", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.3",
				ContainerPath: "/dev/dfl-port.3",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/dfl-port.4",
				ContainerPath: "/dev/dfl-port.4",
				Permissions:   "rw",
			},
		},
	})

	result := getRegionTree(getDevicesDFL())
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Got unexpected result: %v, expected: %v", result, expected)
	}
}

func TestGetAfuTreeDFL(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "dfl-port.0", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.0",
				ContainerPath: "/dev/dfl-port.0",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "dfl-port.1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.1",
				ContainerPath: "/dev/dfl-port.1",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "dfl-port.2", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.2",
				ContainerPath: "/dev/dfl-port.2",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(afMode+"-"+unhealthyAfuID, "dfl-port.3", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.3",
				ContainerPath: "/dev/dfl-port.3",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "dfl-port.4", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/dfl-port.4",
				ContainerPath: "/dev/dfl-port.4",
				Permissions:   "rw",
			},
		},
	})

	result := getAfuTree(getDevicesDFL())
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Got unexpected result: %v, expected: %v", result, expected)
	}
}

func TestScanFPGAsDFL(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/fpgaplugin-TestDiscoverFPGAs-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sys", "class", "fpga_region")
	devfs := path.Join(tmpdir, "dev")
	tcases := []struct {
		name            string
		devfsdirs       []string
		sysfsdirs       []string
		sysfsfiles      map[string][]byte
		errorContains   string
		expectedDevTree map[string]map[string]dpapi.DeviceInfo
		mode            string
	}{
		{
			name: "No sysfs folder exists",
			mode: afMode,
		},
		{
			name: "Unexpected extra fme devices in the region",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-fme.1"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			devfsdirs:     []string{"dfl-fme.0"},
			errorContains: "Detected more than one FPGA region for device region1. Only one region per FPGA device is supported",
		},
		{
			name:          "AFU without ID",
			mode:          afMode,
			sysfsdirs:     []string{"region1/dfl-port.0"},
			errorContains: "/sys/class/fpga_region/region1/dfl-port.0/afu_id: no such file or directory",
		},
		{
			name:      "No device node for detected AFU",
			mode:      afMode,
			sysfsdirs: []string{"region1/dfl-port.0"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "/dev/dfl-port.0: no such file or directory",
		},
		{
			name:      "AFU without corresponding FME",
			mode:      afMode,
			sysfsdirs: []string{"region1/dfl-port.0"},
			devfsdirs: []string{"dfl-port.0"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "region1: AFU without corresponding FME found",
		},
		{
			name: "More than one FME per FPGA device",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-fme.1/dfl-fme-region.1/fpga_region/region1/",
			},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region1/dfl-fme.1/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			devfsdirs: []string{
				"dfl-fme.0",
				"dfl-fme.1",
			},
			errorContains: "Detected more than one FPGA region",
		},
		{
			name:          "No regionX/dfl-fme.k/dfl-fme-region.n entry found",
			mode:          afMode,
			sysfsdirs:     []string{"region1/dfl-fme.0", "region1/dfl-port.0", "region1/dfl-port.1"},
			errorContains: "no compat_id found with pattern ",
		},
		{
			name: "Duplicate regionX/dfl-fme.k/dfl-fme-region.n entry found",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-fme.0/dfl-fme-region.2/fpga_region/region1/",
				"region1/dfl-port.0"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region1/dfl-fme.0/dfl-fme-region.2/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},

			errorContains: "/sys/class/fpga_region/region1/dfl-fme.0/dfl-fme-region.*/fpga_region/region*/compat_id' matches multiple files",
		},
		{
			name: "fme device doesn't exist",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-port.0",
				"region1/dfl-port.1"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			errorContains: "/dev/dfl-fme.0 doesn't exist",
		},
		{
			name: "region1/dfl-port.0/afu_id file doesn't exist",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-port.0",
				"region1/dfl-port.1"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			devfsdirs:     []string{"dfl-fme.0"},
			errorContains: "region1/dfl-port.0/afu_id: no such file or directory",
		},
		{
			name: "region1/dfl-port.0/afu_id is a directory",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-port.0/afu_id",
				"region1/dfl-port.1"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			devfsdirs:     []string{"dfl-fme.0"},
			errorContains: "region1/dfl-port.0/afu_id: is a directory",
		},
		{
			name: "port device doesn't exist",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-port.0",
				"region1/dfl-port.1"},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region1/dfl-port.0/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			devfsdirs:     []string{"dfl-fme.0"},
			errorContains: "/dev/dfl-port.0 doesn't exist",
		},
		{
			name: "working af mode",
			mode: afMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-port.0",
				"region1/dfl-port.1",
				"region2/dfl-fme.1/dfl-fme-region.2/fpga_region/region2/",
				"region2/dfl-port.2",
				"region2/dfl-port.3",
			},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region1/dfl-port.0/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region1/dfl-port.1/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region2/dfl-fme.1/dfl-fme-region.2/fpga_region/region2/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region2/dfl-port.2/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region2/dfl-port.3/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			devfsdirs: []string{"dfl-fme.0", "dfl-port.0", "dfl-port.1", "dfl-fme.1", "dfl-port.2", "dfl-port.3"},
		},
		{
			name: "working region mode",
			mode: regionMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-port.0",
				"region1/dfl-port.1",
				"region2/dfl-fme.1/dfl-fme-region.2/fpga_region/region2/",
				"region2/dfl-port.2",
				"region2/dfl-port.3",
			},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region1/dfl-port.0/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region1/dfl-port.1/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region2/dfl-fme.1/dfl-fme-region.2/fpga_region/region2/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region2/dfl-port.2/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region2/dfl-port.3/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			devfsdirs: []string{"dfl-fme.0", "dfl-port.0", "dfl-port.1", "dfl-fme.1", "dfl-port.2", "dfl-port.3"},
		},
		{
			name: "working regionDevel mode",
			mode: regionDevelMode,
			sysfsdirs: []string{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/",
				"region1/dfl-port.0",
				"region1/dfl-port.1",
				"region2/dfl-fme.1/dfl-fme-region.2/fpga_region/region2/",
				"region2/dfl-port.2",
				"region2/dfl-port.3",
			},
			sysfsfiles: map[string][]byte{
				"region1/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region1/dfl-port.0/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region1/dfl-port.1/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region2/dfl-fme.1/dfl-fme-region.2/fpga_region/region2/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"region2/dfl-port.2/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"region2/dfl-port.3/afu_id":                                        []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			devfsdirs: []string{"dfl-fme.0", "dfl-port.0", "dfl-port.1", "dfl-fme.1", "dfl-port.2", "dfl-port.3"},
		},
	}

	for _, tcase := range tcases {
		err := createTestDirs(devfs, sysfs, tcase.devfsdirs, tcase.sysfsdirs, tcase.sysfsfiles)
		if err != nil {
			t.Fatalf("%+v", err)
		}

		plugin, err := newDevicePluginDFL(sysfs, devfs, tcase.mode)
		if err != nil {
			t.Errorf("Test case '%s': %+v", tcase.name, err)
			continue
		}
		plugin.getDevTree = func(devices []device) dpapi.DeviceTree {
			return dpapi.NewDeviceTree()
		}

		_, err = plugin.scanFPGAs()
		if tcase.errorContains != "" {
			if err == nil || !strings.Contains(err.Error(), tcase.errorContains) {
				t.Errorf("Test case '%s': expected error '%s', but got '%v'", tcase.name, tcase.errorContains, err)
			}
		} else if err != nil {
			t.Errorf("Test case '%s': expected no error, but got '%+v'", tcase.name, err)
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatal(err)
		}
	}
}
