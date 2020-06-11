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
	"io/ioutil"
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
		err = ioutil.WriteFile(path.Join(sysfs, filename), body, 0600)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
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
	root, err := ioutil.TempDir("", "test_new_device_plugin")
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
	root, err := ioutil.TempDir("", "test_new_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}
	defer os.RemoveAll(root)

	devfs := path.Join(root, "dev")

	tcases := []struct {
		name        string
		mode        string
		sysfs       string
		devfsdirs   []string
		sysfsdirs   []string
		sysfsfiles  map[string][]byte
		expectedErr bool
	}{
		{
			name:  "valid OPAE scan",
			mode:  afMode,
			sysfs: path.Join(root, "sys", "class", "fpga"),
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
			expectedErr: false,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			err = createTestDirs(devfs, tc.sysfs, tc.devfsdirs, tc.sysfsdirs, tc.sysfsfiles)
			if err != nil {
				t.Fatal(err)
			}

			plugin, err := newDevicePlugin(tc.mode, root)
			if err != nil {
				t.Fatalf("failed to create a device plugin: %+v", err)
			}

			err = plugin.Scan(&fakeNotifier{plugin.scanDone})

			if tc.expectedErr && err == nil {
				t.Error("unexpected success")
			}
			if !tc.expectedErr && err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			for _, dir := range []string{"sys", "dev"} {
				err = os.RemoveAll(path.Join(root, dir))
				if err != nil {
					t.Fatalf("Failed to remove fake sysfs directory: %+v", err)
				}
			}
		})
	}
}
