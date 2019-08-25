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

func TestNewDevicePluginOPAE(t *testing.T) {
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
		_, err := newDevicePluginOPAE("", "", tcase.mode)
		if tcase.expectedErr && err == nil {
			t.Error("No error generated when creating Cache with invalid parameters")
		}
	}
}

// getDevices returns static list of device structs for testing purposes
func getDevicesOPAE() []device {
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
		{
			name: "intel-fpga-dev.2",
			regions: []region{
				{
					id:          "intel-fpga-fme.2",
					interfaceID: unhealthyInterfaceID,
					devNode:     "/dev/intel-fpga-fme.2",
					afus: []afu{
						{
							id:      "intel-fpga-port.2",
							afuID:   unhealthyAfuID,
							devNode: "/dev/intel-fpga-port.2",
						},
					},
				},
			},
		},
	}
}

func TestGetRegionDevelTreeOPAE(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.0", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.0",
				ContainerPath: "/dev/intel-fpga-port.0",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/intel-fpga-fme.0",
				ContainerPath: "/dev/intel-fpga-fme.0",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.1",
				ContainerPath: "/dev/intel-fpga-port.1",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/intel-fpga-fme.1",
				ContainerPath: "/dev/intel-fpga-fme.1",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "intel-fpga-fme.2", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.2",
				ContainerPath: "/dev/intel-fpga-port.2",
				Permissions:   "rw",
			},
			{
				HostPath:      "/dev/intel-fpga-fme.2",
				ContainerPath: "/dev/intel-fpga-fme.2",
				Permissions:   "rw",
			},
		},
	})

	result := getRegionDevelTree(getDevicesOPAE())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetRegionTreeOPAE(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.0", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.0",
				ContainerPath: "/dev/intel-fpga-port.0",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.1",
				ContainerPath: "/dev/intel-fpga-port.1",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "intel-fpga-fme.2", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.2",
				ContainerPath: "/dev/intel-fpga-port.2",
				Permissions:   "rw",
			},
		},
	})

	result := getRegionTree(getDevicesOPAE())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetAfuTreeOPAE(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "intel-fpga-port.0", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.0",
				ContainerPath: "/dev/intel-fpga-port.0",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "intel-fpga-port.1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.1",
				ContainerPath: "/dev/intel-fpga-port.1",
				Permissions:   "rw",
			},
		},
	})
	expected.AddDevice(afMode+"-"+unhealthyAfuID, "intel-fpga-port.2", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []pluginapi.DeviceSpec{
			{
				HostPath:      "/dev/intel-fpga-port.2",
				ContainerPath: "/dev/intel-fpga-port.2",
				Permissions:   "rw",
			},
		},
	})

	result := getAfuTree(getDevicesOPAE())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestScanFPGAsOPAE(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/fpgaplugin-TestDiscoverFPGAs-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sys", "class", "fpga")
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
			name:          "FPGA device without FME and ports",
			mode:          afMode,
			sysfsdirs:     []string{"intel-fpga-dev.0", "incorrect-file-name"},
			errorContains: "No regions found",
		},
		{
			name:          "AFU without ID",
			mode:          afMode,
			sysfsdirs:     []string{"intel-fpga-dev.0/intel-fpga-port.0"},
			errorContains: "afu_id: no such file or directory",
		},
		{
			name:      "No device node for detected AFU",
			mode:      afMode,
			sysfsdirs: []string{"intel-fpga-dev.0/intel-fpga-port.0"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "/dev/intel-fpga-port.0: no such file or directory",
		},
		{
			name:      "AFU without corresponding FME",
			mode:      afMode,
			sysfsdirs: []string{"intel-fpga-dev.0/intel-fpga-port.0"},
			devfsdirs: []string{"intel-fpga-port.0"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "intel-fpga-dev.0: AFU without corresponding FME found",
		},
		{
			name: "More than one FME per FPGA device",
			mode: afMode,
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
			mode:          afMode,
			sysfsdirs:     []string{"intel-fpga-dev.0/intel-fpga-fme.0"},
			errorContains: "interface_id: no such file or directory",
		},
		{
			name:      "No device node for detected region",
			mode:      afMode,
			sysfsdirs: []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			errorContains: "/dev/intel-fpga-fme.0: no such file or directory",
		},
		{
			name: "No errors expected in af mode",
			mode: afMode,
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
		},
		{
			name: "No errors expected in region mode",
			mode: regionMode,
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
		},
	}

	for _, tcase := range tcases {
		err := createTestDirs(devfs, sysfs, tcase.devfsdirs, tcase.sysfsdirs, tcase.sysfsfiles)
		if err != nil {
			t.Fatalf("%+v", err)
		}

		plugin, err := newDevicePluginOPAE(sysfs, devfs, tcase.mode)

		plugin.getDevTree = func(devices []device) dpapi.DeviceTree {
			return dpapi.NewDeviceTree()
		}

		if err != nil {
			t.Fatalf("%+v", err)
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
