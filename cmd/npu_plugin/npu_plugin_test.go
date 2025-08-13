// Copyright 2017-2023 Intel Corporation. All Rights Reserved.
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
	"testing"

	"github.com/pkg/errors"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

// mockNotifier implements Notifier interface for NPU.
type mockNotifier struct {
	scanDone chan bool
	npuCount int
}

// Notify stops plugin Scan.
func (n *mockNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.npuCount = len(newDeviceTree[deviceTypeNpu])

	n.scanDone <- true
}

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

// mockNotifier implements Notifier interface.

type TestCaseDetails struct {
	// test-case environment
	sysfsfiles map[string][]byte
	name       string
	sysfsdirs  []string
	devfsdirs  []string
	// how plugin should interpret it
	options cliOptions
	// what the result should be (i915)
	expectedDevs int
}

func createTestFiles(root string, tc TestCaseDetails) (string, string, error) {
	sysfs := path.Join(root, "sys")
	devfs := path.Join(root, "dev")

	for _, devfsdir := range tc.devfsdirs {
		if err := os.MkdirAll(path.Join(devfs, devfsdir), 0750); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	if err := os.MkdirAll(sysfs, 0750); err != nil {
		return "", "", errors.Wrap(err, "Failed to create fake base sysfs directory")
	}

	for _, sysfsdir := range tc.sysfsdirs {
		if err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0750); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	for filename, body := range tc.sysfsfiles {
		if err := os.WriteFile(path.Join(sysfs, filename), body, 0600); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	return sysfs, devfs, nil
}

func TestNewDevicePlugin(t *testing.T) {
	if newDevicePlugin("", "", cliOptions{sharedDevNum: 2}) == nil {
		t.Error("Failed to create NPU plugin")
	}
}

func TestAllocate(t *testing.T) {
	plugin := newDevicePlugin("", "", cliOptions{sharedDevNum: 2})

	_, err := plugin.Allocate(&v1beta1.AllocateRequest{})
	if _, ok := err.(*dpapi.UseDefaultMethodError); !ok {
		t.Errorf("Unexpected return value: %+v", err)
	}
}

func TestScan(t *testing.T) {
	tcases := []TestCaseDetails{
		{
			name:         "no sysfs mounted",
			expectedDevs: 0,
		},
		{
			name:         "no device installed",
			sysfsdirs:    []string{"accel0"},
			expectedDevs: 0,
		},
		{
			name:      "missing dev node",
			sysfsdirs: []string{"accel0/device"},
			sysfsfiles: map[string][]byte{
				"accel0/device/vendor": []byte("0x8086"),
			},
			expectedDevs: 0,
		},
		{
			name:      "unknown device",
			sysfsdirs: []string{"accel0/device/drm/accel0", "accel0/device/drm/accelD0"},
			sysfsfiles: map[string][]byte{
				"accel0/device/vendor": []byte("0x8086"),
				"accel0/device/device": []byte("0xffff"),
			},
			devfsdirs: []string{
				"accel0",
			},
			expectedDevs: 0,
		},
		{
			name:      "device id with endline",
			sysfsdirs: []string{"accel0/device/drm/accel0", "accel0/device/drm/accelD0"},
			sysfsfiles: map[string][]byte{
				"accel0/device/vendor": []byte("0x8086"),
				"accel0/device/device": []byte("0x7e4c\n"),
			},
			devfsdirs: []string{
				"accel0",
			},
			expectedDevs: 1,
		},
		{
			name:      "one device",
			sysfsdirs: []string{"accel0/device/drm/accel0", "accel0/device/drm/accelD0"},
			sysfsfiles: map[string][]byte{
				"accel0/device/vendor": []byte("0x8086"),
				"accel0/device/device": []byte("0x643e"),
			},
			devfsdirs: []string{
				"accel0",
			},
			expectedDevs: 1,
		},
		{
			name: "two devices",
			sysfsdirs: []string{
				"accel0/device/drm/accel0", "accel0/device/drm/accelD0",
				"accel1/device/drm/accel1", "accel1/device/drm/accelD1",
			},
			sysfsfiles: map[string][]byte{
				"accel0/device/vendor": []byte("0x8086"),
				"accel0/device/device": []byte("0x643e"),
				"accel1/device/vendor": []byte("0x8086"),
				"accel1/device/device": []byte("0xad1d"),
			},
			devfsdirs: []string{
				"accel0",
				"accel1",
			},
			expectedDevs: 2,
		},
	}

	for _, tc := range tcases {
		if tc.options.sharedDevNum == 0 {
			tc.options.sharedDevNum = 1
		}

		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp("", "test_new_device_plugin")
			if err != nil {
				t.Fatalf("Can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			sysfs, devfs, err := createTestFiles(root, tc)
			if err != nil {
				t.Errorf("Unexpected error: %+v", err)
			}

			plugin := newDevicePlugin(sysfs, devfs, tc.options)

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

			err = plugin.Scan(notifier)
			// Scans in NPU plugin never fail
			if err != nil {
				t.Errorf("Unexpected error: %+v", err)
			}
			if tc.expectedDevs != notifier.npuCount {
				t.Errorf("Expected %d, discovered %d devices (NPU)",
					tc.expectedDevs, notifier.npuCount)
			}
		})
	}
}
