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

package dpdkdrv

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/pkg/errors"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func init() {
	flag.Set("v", "4") //Enable debug output
}

func createTestFiles(prefix string, dirs []string, files map[string][]byte, symlinks map[string]string) error {
	for _, dir := range dirs {

		err := os.MkdirAll(path.Join(prefix, dir), 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}

	}
	for filename, body := range files {

		err := ioutil.WriteFile(path.Join(prefix, filename), body, 0644)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
		}

	}
	for link, target := range symlinks {

		err := os.MkdirAll(path.Join(prefix, target), 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake symlink target directory")
		}
		err = os.Symlink(path.Join(prefix, target), path.Join(prefix, link))
		if err != nil {
			return errors.Wrap(err, "Failed to create fake symlink")
		}
	}
	return nil
}

func TestNewDevicePlugin(t *testing.T) {

	tcases := []struct {
		name            string
		dpdkDriver      string
		kernelVfDrivers string
		expectedErr     bool
	}{
		{
			name:            "Wrong dpdkDriver",
			dpdkDriver:      "uio",
			kernelVfDrivers: "c6xxvf",
			expectedErr:     true,
		},
		{
			name:            "Correct dpdkDriver, wrong kernelVfDrivers",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: "c6xxxvf",
			expectedErr:     true,
		},
		{
			name:            "Correct dpdkDriver, kernelVfDrivers separated wrong",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: "c6xxvf:d15xxvf",
			expectedErr:     true,
		},
		{
			name:            "No errors",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: "c6xxvf,d15xxvf",
			expectedErr:     false,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDevicePlugin(1, tt.kernelVfDrivers, tt.dpdkDriver)

			if tt.expectedErr && err == nil {
				t.Errorf("Test case '%s': expected error", tt.name)
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Test case '%s': expected success", tt.name)
			}
		})
	}
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
		symlinks        map[string]string
		expectedErr     bool
		maxDevNum       int
		expectedDevNum  int
	}{
		{
			name: "No error returned for uninitialized device plugin",
		},
		{
			name:        "Only DPDK driver is set and no devs allowed and vfdevID cannot be determined",
			dpdkDriver:  "igb_uio",
			dirs:        []string{"sys/bus/pci/drivers/igb_uio/0000:test"},
			expectedErr: true,
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
			name:       "igb_uio DPDKdriver with one DPDK bound device where vfdevID cannot be determined",
			dpdkDriver: "igb_uio",
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio/sometestfile",
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
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/device": []byte("some junk"),
			},
			maxDevNum:      1,
			expectedDevNum: 0,
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
			maxDevNum:      1,
			expectedDevNum: 0,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (not QAT device) where vfdevID is not equal to qatDevId (37c9)",
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
			expectedDevNum: 0,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) ",
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
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("0x37c9"),
				"sys/bus/pci/drivers/igb_uio/new_id":             []byte("some junk"),
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) where reading uioDirPath for obtaining device file fails ",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/driver",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("0x37c9"),
				"sys/bus/pci/drivers/igb_uio/new_id":             []byte("some junk"),
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) but no uio device is found",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio",
				"sys/bus/pci/devices/0000:02:00.0/driver",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("0x37c9"),
				"sys/bus/pci/drivers/igb_uio/new_id":             []byte("some junk"),
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) where the available devices on the system are 2 but maxNumDevices=1] ",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.1",
				"sys/bus/pci/devices/0000:02:00.1/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.1/driver",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("0x37c9"),
				"sys/bus/pci/drivers/igb_uio/new_id":             []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.1/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.1/device":        []byte("0x37c9"),
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with one kernel bound device (not QAT device) where vfdevID is not equal to qatDevId (37c9)",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},

			dirs: []string{
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/driver",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/vfio-pci/vfiotestfile",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("some junk"),
				"sys/bus/pci/drivers/vfio-pci/new_id":            []byte("some junk"),
			},
			maxDevNum:      1,
			expectedDevNum: 0,
		},
		{
			name:            "vfio-pci DPDKdriver with  one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9)",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/driver",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/vfio-pci/vfiotestfile",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("0x37c9"),
				"sys/bus/pci/drivers/vfio-pci/new_id":            []byte("some junk"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:00.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with  one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) but symlink is broken",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/drivers/c6xxvf/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/driver",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/vfio-pci/vfiotestfile",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/driver/unbind": []byte("some junk"),
				"sys/bus/pci/devices/0000:02:00.0/device":        []byte("0x37c9"),
				"sys/bus/pci/drivers/vfio-pci/new_id":            []byte("some junk"),
			},
			maxDevNum:   1,
			expectedErr: true,
		},
	}
	for _, tt := range tcases {
		if err := os.MkdirAll(tmpdir, 0755); err != nil {
			t.Fatal(err)
		}

		if err := createTestFiles(tmpdir, tt.dirs, tt.files, tt.symlinks); err != nil {
			t.Fatalf("%+v", err)
		}

		dp := &DevicePlugin{
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
func eleInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
func TestPostAllocate(t *testing.T) {
	response := new(pluginapi.AllocateResponse)
	cresp := new(pluginapi.ContainerAllocateResponse)
	response.ContainerResponses = append(response.ContainerResponses, cresp)
	testMap := map[string]string{
		"QAT29": "03:04.1",
		"QAT13": "03:04.2",
		"QAT6":  "03:04.3",
		"QAT21": "03:04.4",
	}
	response.ContainerResponses[0].Envs = testMap
	resultKey := []string{
		"QAT0",
		"QAT1",
		"QAT2",
		"QAT3",
	}
	expectedValues := map[string]struct{}{
		"03:04.1": {},
		"03:04.2": {},
		"03:04.3": {},
		"03:04.4": {},
	}
	dp := &DevicePlugin{}
	dp.PostAllocate(response)
	if len(response.ContainerResponses[0].Envs) != 4 {
		t.Fatal("Set wrong number of Environment Variables")
	}

	for k := range response.ContainerResponses[0].Envs {
		if !eleInSlice(k, resultKey) {
			t.Fatalf("Set wrong key: %s. The key should be in the range %v", k, resultKey)
		}
	}

	for _, key := range resultKey {
		if value, ok := response.ContainerResponses[0].Envs[key]; ok {
			if _, ok := expectedValues[value]; ok {
				delete(expectedValues, value)
			} else {
				t.Errorf("Unexpected value %s", value)
			}
		}
	}
}
