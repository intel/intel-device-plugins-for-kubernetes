// Copyright 2018-2022 Intel Corporation. All Rights Reserved.
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
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/pkg/errors"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

func createTestFiles(prefix string, dirs []string, files map[string][]byte, symlinks map[string]string) error {
	for _, dir := range dirs {
		err := os.MkdirAll(path.Join(prefix, dir), 0750)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	for filename, body := range files {
		err := os.WriteFile(path.Join(prefix, filename), body, 0600)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	for link, target := range symlinks {
		err := os.MkdirAll(path.Join(prefix, target), 0750)
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
			_, err := NewDevicePlugin(1, tt.kernelVfDrivers, tt.dpdkDriver, "")

			if tt.expectedErr && err == nil {
				t.Errorf("Test case '%s': expected error", tt.name)
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Test case '%s': expected success", tt.name)
			}
		})
	}
}

// fakeNotifier implements Notifier interface.
type fakeNotifier struct {
	scanDone chan bool
	tree     dpapi.DeviceTree
}

// Notify stops plugin Scan.
func (n *fakeNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.tree = newDeviceTree
	n.scanDone <- true
}

func TestGetPreferredAllocation(t *testing.T) {
	rqt := &pluginapi.PreferredAllocationRequest{
		ContainerRequests: []*pluginapi.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"0000:03:00.4", "0000:04:00.1", "0000:05:00.3", "0000:05:00.4", "0000:05:00.1", "0000:04:00.0", "0000:04:00.4", "0000:06:00.4", "0000:04:00.2", "0000:03:00.1", "0000:05:00.0", "0000:05:00.2", "0000:04:00.3", "0000:03:00.2", "0000:06:00.0", "0000:06:00.3", "0000:03:00.3", "0000:03:00.0", "0000:06:00.1", "0000:06:00.2"},
				AllocationSize:     4,
			},
		},
	}

	plugin := newDevicePlugin("", "", 4, []string{""}, "", nonePolicy)
	response, _ := plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"0000:03:00.4", "0000:04:00.1", "0000:05:00.3", "0000:05:00.4"}) {
		t.Error("Unexpected return value for balanced preferred allocation")
	}

	plugin = newDevicePlugin("", "", 4, []string{""}, "", packedPolicy)
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"0000:03:00.0", "0000:03:00.1", "0000:03:00.2", "0000:03:00.3"}) {
		t.Error("Unexpected return value for balanced preferred allocation")
	}

	plugin = newDevicePlugin("", "", 4, []string{""}, "", balancedPolicy)
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"0000:03:00.0", "0000:04:00.0", "0000:05:00.0", "0000:06:00.0"}) {
		t.Error("Unexpected return value for balanced preferred allocation")
	}

	rqt.ContainerRequests[0].AllocationSize = 32
	plugin = newDevicePlugin("", "", 4, []string{""}, "", nil)
	_, err := plugin.GetPreferredAllocation(rqt)

	if err == nil {
		t.Error("Unexpected nil value return for err when AllocationSize is greater than the number of available device IDs")
	}
}

func TestScan(t *testing.T) {
	tcases := []struct {
		name                 string
		dpdkDriver           string
		dirs                 []string
		files                map[string][]byte
		symlinks             map[string]string
		kernelVfDrivers      []string
		expectedErr          bool
		maxDevNum            int
		expectedDevNum       int
		expectedUnhealthyNum int
	}{
		{
			name: "No error returned for uninitialized device plugin",
		},
		{
			name:       "igb_uio DPDKdriver with one DPDK bound device where vfdevID cannot be determined",
			dpdkDriver: "igb_uio",
			dirs: []string{
				"sys/bus/pci/drivers/igb_uio/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:01.0/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.0",
			},
			maxDevNum:      1,
			expectedDevNum: 0,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device, but binding fails",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/driver": "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) where reading uioDirPath for obtaining device file fails ",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":    "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0": "sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:01.0/driver":  "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) but no uio device is found",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/devices/0000:02:01.0/uio",
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":    "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0": "sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:01.0/driver":  "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) where the available devices on the system are 2 but maxNumDevices=1] ",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/devices/0000:02:01.0/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:01.1/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
				"sys/bus/pci/devices/0000:02:01.1/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":    "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0": "sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn1": "sys/bus/pci/devices/0000:02:01.1",
				"sys/bus/pci/devices/0000:02:01.0/driver":  "sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/devices/0000:02:01.1/driver":  "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "igb_uio DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) where the available devices on the system are 2 but maxNumDevices=4] ",
			dpdkDriver:      "igb_uio",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/drivers/igb_uio",
				"sys/bus/pci/devices/0000:02:01.0/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:01.1/uio/sometestfile",
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
				"sys/bus/pci/devices/0000:02:01.1/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":    "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0": "sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn1": "sys/bus/pci/devices/0000:02:01.1",
				"sys/bus/pci/devices/0000:02:01.0/driver":  "sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/devices/0000:02:01.1/driver":  "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:      4,
			expectedDevNum: 2,
		},
		{
			name:            "vfio-pci DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9)",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":        "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0":     "sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:01.0/driver":      "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfdevID is equal to qatDevId (37c9)",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":        "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0":     "sys/bus/pci/devices/0000:02:01.0",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId not enabled in kernelVfDrivers",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/d15xx",
				"sys/bus/pci/drivers/d15xxvf",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x6f55"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/drivers/d15xx/0000:02:00.0":       "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0/driver":      "sys/bus/pci/drivers/d15xxvf",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0":     "sys/bus/pci/devices/0000:02:01.0",
			},
			maxDevNum:      1,
			expectedDevNum: 0,
		},
		{
			name:            "vfio-pci DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9) but symlink is broken",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0/vfio-pci/vfiotestfile",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":    "sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0/virtfn0": "sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:01.0/driver":  "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:   1,
			expectedErr: true,
		},
		{
			name:            "vfio-pci DPDKdriver with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9), running in a VM with vIOMMU",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device": []byte("0x37c9"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/driver":      "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver in unsafe NOIOMMU mode with one kernel bound device (QAT device) where vfdevID is equal to qatDevId (37c9), running in a VM without IOMMU",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xxvf",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:01.0",
				"sys/kernel/iommu_groups/vfiotestfile",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device":   []byte("0x37c9"),
				"sys/kernel/iommu_groups/vfiotestfile/name": []byte("vfio-noiommu"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/driver":      "sys/bus/pci/drivers/c6xxvf",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfdevID is equal to qatDevId (4941), PF with dc capabilities from debugfs",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"4xxxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/4xxx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/devices/pci0000:02/0000:02:00.0",
				"sys/kernel/debug/qat_4xxx_0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/devices/pci0000:02/0000:02:00.0/device":     []byte("0x4940"),
				"sys/kernel/debug/qat_4xxx_0000:02:00.0/dev_cfg": []byte("[GENERAL]\nServicesEnabled = dc"),
				"sys/bus/pci/devices/0000:02:01.0/device":        []byte("0x4941"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/drivers/4xxx/0000:02:00.0":        "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0":             "sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn0":  "sys/bus/pci/devices/0000:02:01.0",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfdevID is equal to qatDevId (4941), PF with cy capabilities from sysfs",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"4xxxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/4xxx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/qat",
				"sys/kernel/debug/qat_4xxx_0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/devices/pci0000:02/0000:02:00.0/device":           []byte("0x4940"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/state":        []byte("up"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/cfg_services": []byte("sym;asym"),
				"sys/bus/pci/devices/0000:02:01.0/device":              []byte("0x4941"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/drivers/4xxx/0000:02:00.0":        "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0":             "sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn0":  "sys/bus/pci/devices/0000:02:01.0",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfdevID is equal to qatDevId (4949), PF with decomp capabilities from sysfs",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"6xxxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/6xxx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/qat",
				"sys/kernel/debug/qat_6xxx_0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/devices/pci0000:02/0000:02:00.0/device":           []byte("0x4948"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/state":        []byte("up"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/cfg_services": []byte("decomp"),
				"sys/bus/pci/devices/0000:02:01.0/device":              []byte("0x4949"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/drivers/6xxx/0000:02:00.0":        "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0":             "sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn0":  "sys/bus/pci/devices/0000:02:01.0",
			},
			maxDevNum:      1,
			expectedDevNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfdevID is equal to qatDevId (4941), two PFs with dc and cy capabilities",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"4xxxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/4xxx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:03/0000:03:00.0",
				"sys/kernel/debug/qat_4xxx_0000:02:00.0",
				"sys/kernel/debug/qat_4xxx_0000:03:00.0",
				"sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:03:01.0",
			},
			files: map[string][]byte{
				"sys/devices/pci0000:02/0000:02:00.0/device":     []byte("0x4940"),
				"sys/devices/pci0000:03/0000:03:00.0/device":     []byte("0x4940"),
				"sys/kernel/debug/qat_4xxx_0000:02:00.0/dev_cfg": []byte("[GENERAL]\nServicesEnabled = dc"),
				"sys/kernel/debug/qat_4xxx_0000:03:00.0/dev_cfg": []byte("[GENERAL]\nServicesEnabled = sym;asym"),
				"sys/bus/pci/devices/0000:02:01.0/device":        []byte("0x4941"),
				"sys/bus/pci/devices/0000:03:01.0/device":        []byte("0x4941"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:03:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile2",
				"sys/bus/pci/devices/0000:02:01.0/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:03:01.0/physfn":      "sys/devices/pci0000:03/0000:03:00.0",
				"sys/bus/pci/drivers/4xxx/0000:02:00.0":        "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/drivers/4xxx/0000:03:00.0":        "sys/devices/pci0000:03/0000:03:00.0",
				"sys/bus/pci/devices/0000:02:00.0":             "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:03:00.0":             "sys/devices/pci0000:03/0000:03:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn0":  "sys/bus/pci/devices/0000:02:01.0",
				"sys/devices/pci0000:03/0000:03:00.0/virtfn0":  "sys/bus/pci/devices/0000:03:01.0",
			},
			maxDevNum:      2,
			expectedDevNum: 2,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfdevID is equal to qatDevId (4941) heartbeat status bad",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"4xxxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/4xxx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/qat",
				"sys/kernel/debug/qat_4xxx_0000:02:00.0",
				"sys/kernel/debug/qat_4xxx_0000:02:00.0/heartbeat",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/devices/pci0000:02/0000:02:00.0/device":              []byte("0x4940"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/state":           []byte("up"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/cfg_services":    []byte("sym;asym"),
				"sys/bus/pci/devices/0000:02:01.0/device":                 []byte("0x4941"),
				"sys/kernel/debug/qat_4xxx_0000:02:00.0/heartbeat/status": []byte("-1"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/drivers/4xxx/0000:02:00.0":        "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0":             "sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn0":  "sys/bus/pci/devices/0000:02:01.0",
				"sys/devices/pci0000:02/0000:02:00.0/driver":   "sys/bus/pci/drivers/4xxx",
			},
			maxDevNum:            1,
			expectedDevNum:       1,
			expectedUnhealthyNum: 1,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfdevID is equal to qatDevId (4941) heartbeat status good",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"4xxxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/4xxx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/qat",
				"sys/kernel/debug/qat_4xxx_0000:02:00.0",
				"sys/kernel/debug/qat_4xxx_0000:02:00.0/heartbeat",
				"sys/bus/pci/devices/0000:02:01.0",
			},
			files: map[string][]byte{
				"sys/devices/pci0000:02/0000:02:00.0/device":              []byte("0x4940"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/state":           []byte("up"),
				"sys/devices/pci0000:02/0000:02:00.0/qat/cfg_services":    []byte("sym;asym"),
				"sys/bus/pci/devices/0000:02:01.0/device":                 []byte("0x4941"),
				"sys/kernel/debug/qat_4xxx_0000:02:00.0/heartbeat/status": []byte("0"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/drivers/4xxx/0000:02:00.0":        "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0":             "sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn0":  "sys/bus/pci/devices/0000:02:01.0",
				"sys/devices/pci0000:02/0000:02:00.0/driver":   "sys/bus/pci/drivers/4xxx",
			},
			maxDevNum:            1,
			expectedDevNum:       1,
			expectedUnhealthyNum: 0,
		},
		{
			name:            "vfio-pci DPDKdriver with no kernel bound driver and where vfDevID is equal to qatDevId (37c9) heartbeat status bad",
			dpdkDriver:      "vfio-pci",
			kernelVfDrivers: []string{"c6xxvf"},
			dirs: []string{
				"sys/bus/pci/drivers/c6xx",
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:01.0",
				"sys/bus/pci/devices/0000:02:01.1",
				"sys/devices/pci0000:02/0000:02:00.0",
				"sys/kernel/debug/qat_c6xx_0000:02:00.0/heartbeat",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:01.0/device":                 []byte("0x37c9"),
				"sys/bus/pci/devices/0000:02:01.1/device":                 []byte("0x37c9"),
				"sys/kernel/debug/qat_c6xx_0000:02:00.0/heartbeat/status": []byte("-1"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:01.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.0/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:01.1/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:01.1/physfn":      "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/drivers/c6xx/0000:02:00.0":        "sys/devices/pci0000:02/0000:02:00.0",
				"sys/bus/pci/devices/0000:02:00.0":             "sys/devices/pci0000:02/0000:02:00.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn0":  "sys/bus/pci/devices/0000:02:01.0",
				"sys/devices/pci0000:02/0000:02:00.0/virtfn1":  "sys/bus/pci/devices/0000:02:01.1",
				"sys/devices/pci0000:02/0000:02:00.0/driver":   "sys/bus/pci/drivers/c6xx",
			},
			maxDevNum:            3,
			expectedDevNum:       2,
			expectedUnhealthyNum: 2,
		},
	}

	countUnhealthyDevices := func(tree dpapi.DeviceTree) int {
		unhealtyNum := 0

		for _, v := range tree {
			for _, vv := range v {
				field := reflect.ValueOf(vv).FieldByName("state")
				if fmt.Sprintf("%+v", field) == pluginapi.Unhealthy {
					unhealtyNum = unhealtyNum + 1
				}
			}
		}

		return unhealtyNum
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir, err := os.MkdirTemp("/tmp/", "qatplugin-TestScanPrivate-*")
			if err != nil {
				t.Fatal(err)
			}

			if err = createTestFiles(tmpdir, tt.dirs, tt.files, tt.symlinks); err != nil {
				t.Fatalf("%+v", err)
			}

			dp := newDevicePlugin(
				path.Join(tmpdir, "sys/bus/pci/drivers"),
				path.Join(tmpdir, "sys/bus/pci/devices"),
				tt.maxDevNum,
				tt.kernelVfDrivers,
				tt.dpdkDriver,
				nil,
			)

			fN := fakeNotifier{
				scanDone: dp.scanDone,
			}

			err = dp.Scan(&fN)

			if tt.expectedErr && err == nil {
				t.Errorf("expected error, but got success")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("got unexpected error: %+v", err)
			}
			devNum := 0
			for _, resource := range fN.tree {
				devNum = devNum + len(resource)
			}
			if devNum != tt.expectedDevNum {
				t.Errorf("expected %d, but got %d devices", tt.expectedDevNum, devNum)
			}

			if unhealtyNum := countUnhealthyDevices(fN.tree); unhealtyNum != tt.expectedUnhealthyNum {
				t.Errorf("expected %d, but got %d unhealthy devices", tt.expectedUnhealthyNum, unhealtyNum)
			}

			if err = os.RemoveAll(tmpdir); err != nil {
				t.Fatal(err)
			}
		})
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
	if err := dp.PostAllocate(response); err != nil {
		t.Errorf("Unexpected error: %+v", err)
	}

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
