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
	"flag"
	"io/ioutil"
	"os"
	"path"
	"testing"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

func init() {
	flag.Set("v", "4") //Enable debug output
}

// mockNotifier implements Notifier interface.
type mockNotifier struct {
	scanDone chan bool
	devCount int
}

// Notify stops plugin Scan.
func (n *mockNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.devCount = len(newDeviceTree[deviceType])
	n.scanDone <- true
}

func TestScan(t *testing.T) {
	root, err := ioutil.TempDir("", "test_new_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	sysfs := path.Join(root, "sys")
	devfs := path.Join(root, "dev")

	tcases := []struct {
		name         string
		devfsdirs    []string
		sysfsdirs    []string
		sysfsfiles   map[string][]byte
		expectedDevs int
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
			name:      "all is correct",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
			devfsdirs:    []string{"card0"},
			expectedDevs: 1,
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
			name:      "wrong vendor",
			sysfsdirs: []string{"card0/device/drm/card0"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0xbeef"),
			},
			devfsdirs: []string{"card0"},
		},
		{
			name:      "no sysfs records",
			sysfsdirs: []string{"non_gpu_card"},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			for _, devfsdir := range tc.devfsdirs {
				if err := os.MkdirAll(path.Join(devfs, devfsdir), 0755); err != nil {
					t.Fatalf("Failed to create fake device directory: %+v", err)
				}
			}
			for _, sysfsdir := range tc.sysfsdirs {
				if err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0755); err != nil {
					t.Fatalf("Failed to create fake device directory: %+v", err)
				}
			}
			for filename, body := range tc.sysfsfiles {
				if err := ioutil.WriteFile(path.Join(sysfs, filename), body, 0644); err != nil {
					t.Fatalf("Failed to create fake vendor file: %+v", err)
				}
			}

			plugin := newDevicePlugin(sysfs, devfs, 1)

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

			err := plugin.Scan(notifier)
			// Scans in GPU plugin never fail
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if tc.expectedDevs != notifier.devCount {
				t.Errorf("Wrong number of discovered devices")
			}
		})
	}
}
