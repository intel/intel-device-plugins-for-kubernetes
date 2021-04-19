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
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/pkg/errors"

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
		if err := ioutil.WriteFile(path.Join(sysfs, filename), body, 0600); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake vendor file")
		}
	}
	return sysfs, devfs, nil
}

func TestScan(t *testing.T) {
	tcases := []struct {
		name             string
		devfsdirs        []string
		sysfsdirs        []string
		sysfsfiles       map[string][]byte
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
			devfsdirs:        []string{"card0"},
			expectedDevs:     1,
			expectedMonitors: 1,
		},
		{
			name:      "sriov-1-pf-no-vfs",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":       []byte("0x8086"),
				"card0/device/sriov_numvfs": []byte("0"),
			},
			devfsdirs:        []string{"card0"},
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
			devfsdirs:        []string{"card0"},
			expectedDevs:     1,
			expectedMonitors: 1,
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
			devfsdirs:        []string{"card0", "card1", "card2"},
			expectedDevs:     2,
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
			name:      "no sysfs records",
			sysfsdirs: []string{"non_gpu_card"},
		},
	}

	opts := cliOptions{sharedDevNum: 1}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := ioutil.TempDir("", "test_new_device_plugin")
			if err != nil {
				t.Fatalf("can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			sysfs, devfs, err := createTestFiles(root, tc.devfsdirs, tc.sysfsdirs, tc.sysfsfiles)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			plugin := newDevicePlugin(sysfs, devfs, opts)

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

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
