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
	"flag"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/pkg/errors"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

func init() {
	_ = flag.Set("v", "4") // Enable debug output
}

func createTestDirs(devfs, sysfs string, devfsDirs, sysfsDirs []string, sysfsFiles map[string][]byte) error {
	var err error

	for _, devfsdir := range devfsDirs {
		err = os.MkdirAll(path.Join(devfs, devfsdir), 0750)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}
	for _, sysfsdir := range sysfsDirs {
		err = os.MkdirAll(path.Join(sysfs, sysfsdir), 0750)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}
	for filename, body := range sysfsFiles {
		err = os.WriteFile(path.Join(sysfs, filename), body, 0600)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	return nil
}

// validateDevTree is a helper that reduces code complexity to make golangci-lint happy.
func validateDevTree(expectedDevTreeKeys map[string][]string, devTree dpapi.DeviceTree) error {
	for resource, devices := range expectedDevTreeKeys {
		if _, ok := devTree[resource]; !ok {
			return fmt.Errorf("device tree: resource %s missing", resource)
		}
		for _, device := range devices {
			if _, ok := devTree[resource][device]; !ok {
				return fmt.Errorf("device tree resource %s: device %s missing", resource, device)
			}
		}
	}
	return nil
}

func TestPostAllocate(t *testing.T) {
	response := new(pluginapi.AllocateResponse)
	cresp := new(pluginapi.ContainerAllocateResponse)
	response.ContainerResponses = append(response.ContainerResponses, cresp)

	testValue := "some value"

	dp := &devicePlugin{
		annotationValue: testValue,
	}
	if err := dp.PostAllocate(response); err != nil {
		t.Errorf("Unexpected error: %+v", err)
	}

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

func TestNewDevicePlugin(t *testing.T) {
	root, err := os.MkdirTemp("", "test_new_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}
	defer os.RemoveAll(root)

	tcases := []struct {
		name        string
		mode        string
		sysfs       string
		sysfsdirs   []string
		expectedErr bool
	}{
		{
			name:        "valid OPAE af mode",
			mode:        afMode,
			sysfs:       path.Join(root, "sys", "class", "fpga"),
			sysfsdirs:   []string{"intel-fpga-dev.0"},
			expectedErr: false,
		},
		{
			name:        "valid DFL af mode",
			mode:        afMode,
			sysfs:       path.Join(root, "sys", "class", "fpga_region"),
			sysfsdirs:   []string{"dfl-port.0"},
			expectedErr: false,
		},
		{
			name:        "invalid: af mode: driver is not loaded",
			mode:        afMode,
			sysfs:       root,
			expectedErr: true,
		},
		{
			name:        "invalid: region mode: driver is not loaded",
			mode:        regionMode,
			sysfs:       root,
			expectedErr: true,
		},
		{
			name:        "invalid: regionDevel mode: driver is not loaded",
			mode:        regionDevelMode,
			sysfs:       root,
			expectedErr: true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			err = createTestDirs("", tc.sysfs, nil, tc.sysfsdirs, nil)
			if err != nil {
				t.Fatal(err)
			}

			_, err := newDevicePlugin(tc.mode, root)
			if tc.expectedErr && err == nil {
				t.Error("unexpected success")
			}
			if !tc.expectedErr && err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			err = os.RemoveAll(path.Join(root, "sys"))
			if err != nil {
				t.Fatalf("Failed to remove fake sysfs directory: %+v", err)
			}
		})
	}
}

// fakeNotifier implements Notifier interface.
type fakeNotifier struct {
	scanDone chan bool
}

// Notify stops plugin Scan.
func (n *fakeNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.scanDone <- true
}

func TestScan(t *testing.T) {
	root, err := os.MkdirTemp("", "TestScan")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}
	defer os.RemoveAll(root)

	sysfs := path.Join(root, "sys")
	dev := path.Join(root, "dev")

	tcases := []struct {
		name       string
		mode       string
		devs       []string
		sysfsdirs  []string
		sysfsfiles map[string][]byte
		newPort    newPortFunc
	}{
		{
			name: "valid OPAE scan in af mode",
			mode: afMode,
			devs: []string{
				"intel-fpga-fme.0", "intel-fpga-port.0",
				"intel-fpga-fme.1", "intel-fpga-port.1",
			},
			sysfsdirs: []string{
				"class/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"class/fpga/intel-fpga-dev.1/intel-fpga-port.1",
				"class/fpga/dir", // this should be skipped by plugin.ScanFPGAs
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-port.1/",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr",
			},
			sysfsfiles: map[string][]byte{
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-port.1/afu_id": []byte("f7df405cbd7acf7222f144b0b93acd18\n"),

				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			newPort: genNewIntelFpgaPort(sysfs, dev,
				map[string][]string{
					"intel-fpga-port.0": {"devices/pci0000:00/0000:00:03.2/0000:06:00.0", "intel-fpga-fme.0", "devices/pci0000:00/0000:00:03.2/0000:06:00.0"},
					"intel-fpga-port.1": {"devices/pci0000:40/0000:40:02.0/0000:42:00.0", "intel-fpga-fme.1", "devices/pci0000:40/0000:40:02.0/0000:42:00.0"},
				}),
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			err := createTestDirs(dev, sysfs, tc.devs, tc.sysfsdirs, tc.sysfsfiles)
			if err != nil {
				t.Fatalf("%+v", err)
			}

			plugin, err := newDevicePlugin(tc.mode, root)
			if err != nil {
				t.Fatalf("failed to create a device plugin: %+v", err)
			}

			plugin.newPort = tc.newPort

			err = plugin.Scan(&fakeNotifier{plugin.scanDone})

			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			err = os.RemoveAll(root)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
