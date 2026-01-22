// Copyright 2026 Intel Corporation. All Rights Reserved.
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

package vfio

import (
	"flag"
	"os"
	"path"
	"slices"
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

func TestScan(t *testing.T) {
	tcases := []struct {
		name           string
		deviceIDSet    DeviceIDSet
		files          map[string][]byte
		symlinks       map[string]string
		dirs           []string
		expectedDevNum int
		expectedErr    bool
	}{
		{
			name: "No error returned for uninitialized device plugin",
		},
		{
			name:        "PCI Device ID reading fails",
			deviceIDSet: DeviceIDSet{"0x37c8": {}},
			dirs: []string{
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/nodevice": []byte("0x37c8"),
			},
			expectedErr: true,
		},
		{
			name:        "One PCI Device bound to vfio-pci and the Device ID matches",
			deviceIDSet: DeviceIDSet{"0x37c8": {}},
			dirs: []string{
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/device": []byte("0x37c8"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:00.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver":      "sys/bus/pci/drivers/vfio-pci",
			},
			expectedDevNum: 1,
		},
		{
			name:        "PCI Device ID matches but the device is not bound to vfio-pci",
			deviceIDSet: DeviceIDSet{"0x1212": {}},
			dirs: []string{
				"sys/bus/pci/drivers/idxd",
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/device": []byte("0x1212"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:00.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver":      "sys/bus/pci/drivers/idxd",
			},
		},
		{
			name:        "PCI Device ID matches but the device is not bound to any driver",
			deviceIDSet: DeviceIDSet{"0x1212": {}},
			dirs: []string{
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/device": []byte("0x1212"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:00.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
			},
			expectedErr: true,
		},
		{
			name:        "One PCI Device bound to vfio-pci but the Device ID does not match",
			deviceIDSet: DeviceIDSet{"0x37c8": {}},
			dirs: []string{
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/device": []byte("0x4940"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:00.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver":      "sys/bus/pci/drivers/vfio-pci",
			},
		},
		{
			name:        "PCI Device does not belong to any IOMMU group",
			deviceIDSet: DeviceIDSet{"0x37c8": {}},
			dirs: []string{
				"sys/bus/pci/devices/0000:02:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/device": []byte("0x37c8"),
			},
			expectedErr: true,
		},
		{
			name:        "Two PCI Devices bound to vfio-pci but only one Device ID matches",
			deviceIDSet: DeviceIDSet{"0x37c8": {}},
			dirs: []string{
				"sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:02:00.0",
				"sys/bus/pci/devices/0000:03:00.0",
			},
			files: map[string][]byte{
				"sys/bus/pci/devices/0000:02:00.0/device": []byte("0x37c8"),
				"sys/bus/pci/devices/0000:03:00.0/device": []byte("0x4940"),
			},
			symlinks: map[string]string{
				"sys/bus/pci/devices/0000:02:00.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:02:00.0/driver":      "sys/bus/pci/drivers/vfio-pci",
				"sys/bus/pci/devices/0000:03:00.0/iommu_group": "sys/kernel/iommu_groups/vfiotestfile",
				"sys/bus/pci/devices/0000:03:00.0/driver":      "sys/bus/pci/drivers/vfio-pci",
			},
			expectedDevNum: 1,
		},
	}

	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir, err := os.MkdirTemp("/tmp/", "vfioplugin-TestScanPrivate-*")
			if err != nil {
				t.Fatal(err)
			}

			if err = createTestFiles(tmpdir, tt.dirs, tt.files, tt.symlinks); err != nil {
				t.Fatalf("%+v", err)
			}

			dp := NewDevicePlugin(
				path.Join(tmpdir, "sys/bus/pci/devices"),
				tt.deviceIDSet,
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

			if err = os.RemoveAll(tmpdir); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPostAllocate(t *testing.T) {
	response := new(pluginapi.AllocateResponse)
	cresp := new(pluginapi.ContainerAllocateResponse)
	response.ContainerResponses = append(response.ContainerResponses, cresp)
	testMap := map[string]string{
		"VFIO_BDF29": "03:04.1",
		"VFIO_BDF13": "03:04.2",
		"VFIO_BDF6":  "03:04.3",
		"VFIO_BDF21": "03:04.4",
	}
	response.ContainerResponses[0].Envs = testMap
	resultKey := []string{
		"VFIO_BDF0",
		"VFIO_BDF1",
		"VFIO_BDF2",
		"VFIO_BDF3",
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
		if !slices.Contains(resultKey, k) {
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
