// Copyright 2021 Intel Corporation. All Rights Reserved.
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

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/pkg/errors"
)

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

// mockNotifier implements Notifier interface.
type mockNotifier struct {
	scanDone chan bool
	devCount int
}

// Notify stops plugin Scan.
func (n *mockNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.devCount = len(newDeviceTree[deviceTypePF]) + len(newDeviceTree[deviceTypeVF])
	n.scanDone <- true
}

func createTestFiles(devfs string, devfsdirs []string, sysfs string, pfDevs []string, sriovnumvfs []string) error {
	for _, devfsdir := range devfsdirs {
		if err := os.MkdirAll(path.Join(devfs, devfsdir), 0750); err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}

		if err := os.MkdirAll(path.Join(sysfs, devfsdir), 0750); err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	for index, pfDev := range pfDevs {
		if err := os.MkdirAll(path.Join(sysfs, pfDev, "device"), 0750); err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
		if err := os.WriteFile(path.Join(sysfs, pfDev, "device", "sriov_numvfs"), []byte(sriovnumvfs[index]), 0600); err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	return nil
}

func TestNewDevicePlugin(t *testing.T) {
	if NewDevicePlugin("", "") == nil {
		t.Error("Failed to create plugin")
	}
}

func TestScan(t *testing.T) {
	tcases := []struct {
		name string
		// test-case environment
		devfsdirs   []string
		pfDevs      []string
		sriovnumvfs []string
		// what the result should be
		expectedDevs int
	}{
		{
			name:         "no device",
			expectedDevs: 0,
		},
		{
			name:         "pf devices",
			devfsdirs:    []string{"dlb1", "dlb2", "dlb4"},
			pfDevs:       []string{"dlb1", "dlb2", "dlb4"},
			sriovnumvfs:  []string{"0", "0", "0"},
			expectedDevs: 3,
		},
		{
			name:         "vf devices",
			devfsdirs:    []string{"dlb1", "dlb2", "dlb4"},
			expectedDevs: 3,
		},
		{
			name:         "pf & vf devices (derived from another pf)",
			devfsdirs:    []string{"dlb1", "dlb2", "dlb3"},
			pfDevs:       []string{"dlb1"},
			sriovnumvfs:  []string{"0"},
			expectedDevs: 3,
		},
		{
			name:         "pf & vf devices (derived from the pf)",
			devfsdirs:    []string{"dlb0", "dlb2", "dlb3"},
			pfDevs:       []string{"dlb0"},
			sriovnumvfs:  []string{"1"},
			expectedDevs: 2,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp("", "test_new_device_plugin")
			if err != nil {
				t.Fatalf("can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			devfs := path.Join(root, "dev")
			sysfs := path.Join(root, sysfsDir)
			err = createTestFiles(devfs, tc.devfsdirs, sysfs, tc.pfDevs, tc.sriovnumvfs)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			devfs = path.Join(devfs, "dlb*")
			plugin := NewDevicePlugin(devfs, sysfs)

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

			err = plugin.Scan(notifier)
			// Scans in DLB plugin never fail
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if tc.expectedDevs != notifier.devCount {
				t.Errorf("Expected %d, discovered %d devices",
					tc.expectedDevs, notifier.devCount)
			}
		})
	}
}
