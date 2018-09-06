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
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

func init() {
	debug.Activate()
}

func createTestDirs(devfs, sysfs string, devfsDirs, sysfsDirs []string, sysfsFiles map[string][]byte) error {
	var err error

	for _, devfsdir := range devfsDirs {
		err = os.MkdirAll(path.Join(devfs, devfsdir), 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}
	for _, sysfsdir := range sysfsDirs {
		err = os.MkdirAll(path.Join(sysfs, sysfsdir), 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}
	for filename, body := range sysfsFiles {
		err = ioutil.WriteFile(path.Join(sysfs, filename), body, 0644)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	return nil
}

func TestNewDevicePlugin(t *testing.T) {
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
		_, err := newDevicePlugin("", "", tcase.mode)
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

func TestGetRegionDevelTree(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.0", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []string{"/dev/intel-fpga-port.0", "/dev/intel-fpga-fme.0"},
	})
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []string{"/dev/intel-fpga-port.1", "/dev/intel-fpga-fme.1"},
	})
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "intel-fpga-fme.2", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []string{"/dev/intel-fpga-port.2", "/dev/intel-fpga-fme.2"},
	})

	result := getRegionDevelTree(getDevices())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetRegionTree(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.0", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []string{"/dev/intel-fpga-port.0"},
	})
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []string{"/dev/intel-fpga-port.1"},
	})
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "intel-fpga-fme.2", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []string{"/dev/intel-fpga-port.2"},
	})

	result := getRegionTree(getDevices())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetAfuTree(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "intel-fpga-port.0", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []string{"/dev/intel-fpga-port.0"},
	})
	expected.AddDevice(afMode+"-d8424dc4a4a3c413f89e433683f9040b", "intel-fpga-port.1", dpapi.DeviceInfo{
		State: pluginapi.Healthy,
		Nodes: []string{"/dev/intel-fpga-port.1"},
	})
	expected.AddDevice(afMode+"-"+unhealthyAfuID, "intel-fpga-port.2", dpapi.DeviceInfo{
		State: pluginapi.Unhealthy,
		Nodes: []string{"/dev/intel-fpga-port.2"},
	})

	result := getAfuTree(getDevices())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
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
			errorContains: "No regions found",
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

		plugin, err := newDevicePlugin(sysfs, devfs, tcase.mode)
		if err != nil {
			t.Fatalf("%+v", err)
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

func TestPostAllocate(t *testing.T) {
	response := new(pluginapi.AllocateResponse)
	cresp := new(pluginapi.ContainerAllocateResponse)
	response.ContainerResponses = append(response.ContainerResponses, cresp)

	testValue := "some value"

	dp := &devicePlugin{
		annotationValue: testValue,
	}
	dp.PostAllocate(response)

	if len(response.ContainerResponses[0].Annotations) != 1 {
		t.Fatal("Set wrong number of annotations")
	}

	annotation, ok := response.ContainerResponses[0].Annotations[annotationName]
	if ok == false {
		t.Fatalf("%s annotation is not set", annotationName)
	}

	if annotation != testValue {
		t.Fatalf("Set wrong annotation %s", annotation)
	}
}
