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
	"testing"
	"time"
)

func createTestFiles(prefix string, dirs []string, files map[string][]byte) error {
	for _, dir := range dirs {
		err := os.MkdirAll(path.Join(prefix, dir), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create fake device directory: %v", err)
		}
	}
	for filename, body := range files {
		err := ioutil.WriteFile(path.Join(prefix, filename), body, 0644)
		if err != nil {
			return fmt.Errorf("Failed to create fake vendor file: %v", err)
		}
	}

	return nil
}

func TestScanPrivate(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/qatplugin-TestScanPrivate-%d", time.Now().Unix())
	pciDrvDir := path.Join(tmpdir, "sys/bus/pci/drivers")
	pciDevDir := path.Join(tmpdir, "sys/bus/pci/devices")
	tcases := []struct {
		name            string
		dpdkDriver      string
		kernelVfDrivers []string
		dirs            []string
		files           map[string][]byte
		expectedErr     bool
		maxDevNum       int
		expectedDevNum  int
	}{
		{
			name:        "Return error for uninitialized device plugin",
			expectedErr: true,
		},
		{
			name:       "Only DPDK driver is set and no devs allowed",
			dpdkDriver: "igb_uio",
			dirs:       []string{"sys/bus/pci/drivers/igb_uio/0000:test"},
		},
		{
			name:       "Only DPDK driver is set and no dev exists",
			dpdkDriver: "igb_uio",
			dirs:       []string{"sys/bus/pci/drivers/igb_uio/1111:test"},
		},
		{
			name:        "igb_uio DPDK driver with no valid DPDK device under uio directory",
			dpdkDriver:  "igb_uio",
			dirs:        []string{"sys/bus/pci/drivers/igb_uio/0000:02:00.0"},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:       "igb_uio DPDKdriver with no DPDK bound devices",
			dpdkDriver: "igb_uio",
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio",
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:       "igb_uio DPDKdriver with one DPDK bound device",
			dpdkDriver: "igb_uio",
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio/sometestfile",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device, but unbindable",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio/sometestfile",
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device which gets lost after unbinding",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "Broken igb_uio DPDKdriver with one kernel bound device",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("some junk"),
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("some junk"),
				"sys/bus/pci/drivers/igb_uio/new_id":             []byte("some junk"),
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
	}

	for _, tt := range tcases {

		if err := os.MkdirAll(tmpdir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := createTestFiles(tmpdir, tt.dirs, tt.files); err != nil {
			t.Fatal(err)
		}

		dp := &devicePlugin{
			maxDevices:      tt.maxDevNum,
			pciDriverDir:    pciDrvDir,
			pciDeviceDir:    pciDevDir,
			dpdkDriver:      tt.dpdkDriver,
			kernelVfDrivers: tt.kernelVfDrivers,
		}

		tree, err := dp.scan()
		if tt.expectedErr && err == nil {
			t.Errorf("Test case '%s': expected error, but got success", tt.name)
		}
		if !tt.expectedErr && err != nil {
			t.Errorf("Test case '%s': got unexpected error: %+v", tt.name, err)
		}
		if len(tree) != tt.expectedDevNum {
			t.Errorf("Test case '%s': expected %d, but got %d devices", tt.name, tt.expectedDevNum, len(tree))
		}

		if err = os.RemoveAll(tmpdir); err != nil {
			t.Fatal(err)
		}
	}
}
