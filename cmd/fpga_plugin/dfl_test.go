// Copyright 2019 Intel Corporation. All Rights Reserved.
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
	"os"
	"path"
	"reflect"
	"testing"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func TestNewDevicePluginDFL(t *testing.T) {
	tcases := []struct {
		mode        string
		expectedErr bool
	}{
		{
			mode: afMode,
		},
		{
			mode: regionMode,
		},
		{
			mode: regionDevelMode,
		},
		{
			mode:        "unparsable",
			expectedErr: true,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.mode, func(t *testing.T) {
			_, err := newDevicePluginDFL("", "", tcase.mode)
			if tcase.expectedErr && err == nil {
				t.Error("Unexpected success")
			}
			if !tcase.expectedErr && err != nil {
				t.Errorf("Unexpected error: %+v", err)
			}
		})
	}
}

// getDevices returns static list of device structs for testing purposes.
func getDevicesDFL() []device {
	return []device{
		{
			name: "region1",
			regions: []region{
				{
					id:          "region1",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/dfl-fme.0",
					afus: []afu{
						{
							id:      "dfl-port.0",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/dfl-port.0",
						},
					},
				},
			},
		},
		{
			name: "region2",
			regions: []region{
				{
					id:          "region2",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/dfl-fme.1",
					afus: []afu{
						{
							id:      "dfl-port.1",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/dfl-port.1",
						},
						{
							id:      "dfl-port.2",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/dfl-port.2",
						},
					},
				},
			},
		},
		{
			name: "region3",
			regions: []region{
				{
					id:          "region3",
					interfaceID: unhealthyInterfaceID,
					devNode:     "/dev/dfl-fme.2",
					afus: []afu{
						{
							id:      "dfl-port.3",
							afuID:   unhealthyAfuID,
							devNode: "/dev/dfl-port.3",
						},
						{
							id:      "dfl-port.4",
							afuID:   unhealthyAfuID,
							devNode: "/dev/dfl-port.4",
						},
					},
				},
			},
		},
	}
}

func TestGetRegionDevelTreeDFL(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	nodes := []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.0",
			ContainerPath: "/dev/dfl-port.0",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/dfl-fme.0",
			ContainerPath: "/dev/dfl-fme.0",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region1", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.1",
			ContainerPath: "/dev/dfl-port.1",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/dfl-port.2",
			ContainerPath: "/dev/dfl-port.2",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/dfl-fme.1",
			ContainerPath: "/dev/dfl-fme.1",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region2", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.3",
			ContainerPath: "/dev/dfl-port.3",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/dfl-port.4",
			ContainerPath: "/dev/dfl-port.4",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/dfl-fme.2",
			ContainerPath: "/dev/dfl-fme.2",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "region3", dpapi.NewDeviceInfo(pluginapi.Unhealthy, nodes, nil, nil, nil))

	result := getRegionDevelTree(getDevicesDFL())
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Got unexpected result: %v, expected: %v", result, expected)
	}
}

func TestGetRegionTreeDFL(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	nodes := []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.0",
			ContainerPath: "/dev/dfl-port.0",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region1", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.1",
			ContainerPath: "/dev/dfl-port.1",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/dfl-port.2",
			ContainerPath: "/dev/dfl-port.2",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "region2", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.3",
			ContainerPath: "/dev/dfl-port.3",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/dfl-port.4",
			ContainerPath: "/dev/dfl-port.4",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "region3", dpapi.NewDeviceInfo(pluginapi.Unhealthy, nodes, nil, nil, nil))

	result := getRegionTree(getDevicesDFL())
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Got unexpected result: %v, expected: %v", result, expected)
	}
}

func TestGetAfuTreeDFL(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	nodes := []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.0",
			ContainerPath: "/dev/dfl-port.0",
			Permissions:   "rw",
		},
	}
	expected.AddDevice("af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs", "dfl-port.0", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.1",
			ContainerPath: "/dev/dfl-port.1",
			Permissions:   "rw",
		},
	}

	expected.AddDevice("af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs", "dfl-port.1", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.2",
			ContainerPath: "/dev/dfl-port.2",
			Permissions:   "rw",
		},
	}
	expected.AddDevice("af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs", "dfl-port.2", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.3",
			ContainerPath: "/dev/dfl-port.3",
			Permissions:   "rw",
		},
	}
	expected.AddDevice("af-fff.fff.__________________________________________8", "dfl-port.3", dpapi.NewDeviceInfo(pluginapi.Unhealthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/dfl-port.4",
			ContainerPath: "/dev/dfl-port.4",
			Permissions:   "rw",
		},
	}
	expected.AddDevice("af-fff.fff.__________________________________________8", "dfl-port.4", dpapi.NewDeviceInfo(pluginapi.Unhealthy, nodes, nil, nil, nil))

	result := getAfuTree(getDevicesDFL())
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Got unexpected result:\n%v\nexpected:\n%v", result, expected)
	}
}

// genNewDFLPort generates newPortFunc function.
func genNewDFLPort(sysFsPrefix, devDir string, sysFsInfo map[string][]string) newPortFunc {
	return func(portName string) (fpga.Port, error) {
		portPciDev := sysFsInfo[portName][0]
		portPciDevSysFs := path.Join(sysFsPrefix, portPciDev)

		fmeName := sysFsInfo[portName][1]
		fmePciDev := sysFsInfo[portName][2]
		fmePciDevSysFs := path.Join(sysFsPrefix, fmePciDev)

		return &fpga.DflPort{
			Name:      portName,
			DevPath:   path.Join(devDir, portName),
			SysFsPath: path.Join(portPciDevSysFs, "fpga_region", "region*", portName),
			FME: &fpga.DflFME{
				Name:      fmeName,
				DevPath:   path.Join(devDir, fmeName),
				SysFsPath: fmePciDevSysFs,
				PCIDevice: &fpga.PCIDevice{SysFsPath: fmePciDevSysFs},
			},
		}, nil
	}
}
func TestScanFPGAsDFL(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "TestScanFPGAsDFL")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	sysfs := path.Join(tmpdir, "sys")
	dev := path.Join(tmpdir, "dev")
	tcases := []struct {
		sysfsdirs           []string
		sysfsfiles          map[string][]byte
		newPort             newPortFunc
		expectedDevTreeKeys map[string][]string
		name                string
		mode                string
		devs                []string
	}{
		{
			name: "Valid DFL scan in af mode",
			mode: afMode,
			devs: []string{
				"dfl-fme.0", "dfl-port.0",
				"dfl-fme.1", "dfl-port.1",
			},
			sysfsdirs: []string{
				"class/fpga_region/region0/dfl-port.0",
				"class/fpga_region/region1/dfl-port.1",
				"class/fpga_region/dir", // this should be skipped by plugin.ScanFPGAs
				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-port.0",
				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-fme.0/dfl-fme-region.0/fpga_region/region0",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-port.1",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-fme.1/dfl-fme-region.1/fpga_region/region1",
			},
			sysfsfiles: map[string][]byte{
				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-port.1/afu_id": []byte("f7df405cbd7acf7222f144b0b93acd18\n"),

				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-fme.0/dfl-fme-region.0/fpga_region/region0/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-fme.1/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			newPort: genNewDFLPort(sysfs, dev,
				map[string][]string{
					"dfl-port.0": {"devices/pci0000:80/0000:80:01.0/0000:81:00.0", "dfl-fme.0", "devices/pci0000:80/0000:80:01.0/0000:81:00.0"},
					"dfl-port.1": {"devices/pci0000:40/0000:40:02.0/0000:42:00.0", "dfl-fme.1", "devices/pci0000:40/0000:40:02.0/0000:42:00.0"},
				}),
			expectedDevTreeKeys: map[string][]string{
				"af-695.d84.aVKNtusxV3qMNmj5-qCB9thCTcSko8QT-J5DNoP5BAs": {"dfl-port.0"},
				"af-695.f7d.aVKNtusxV3qMNmj5-qCB9vffQFy9es9yIvFEsLk6zRg": {"dfl-port.1"},
			},
		},
		{
			name: "Valid DFL scan in region mode",
			mode: regionMode,
			devs: []string{
				"dfl-fme.0", "dfl-port.0",
				"dfl-fme.1", "dfl-port.1",
			},
			sysfsdirs: []string{
				"class/fpga_region/region0/dfl-port.0",
				"class/fpga_region/region1/dfl-port.1",
				"class/fpga_region/dir", // this should be skipped by plugin.ScanFPGAs
				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-port.0",
				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-fme.0/dfl-fme-region.0/fpga_region/region0",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-port.1",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-fme.1/dfl-fme-region.1/fpga_region/region1",
			},
			sysfsfiles: map[string][]byte{
				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-port.1/afu_id": []byte("f7df405cbd7acf7222f144b0b93acd18\n"),

				"devices/pci0000:80/0000:80:01.0/0000:81:00.0/fpga_region/region0/dfl-fme.0/dfl-fme-region.0/fpga_region/region0/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga_region/region1/dfl-fme.1/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			newPort: genNewDFLPort(sysfs, dev,
				map[string][]string{
					"dfl-port.0": {"devices/pci0000:80/0000:80:01.0/0000:81:00.0", "dfl-fme.0", "devices/pci0000:80/0000:80:01.0/0000:81:00.0"},
					"dfl-port.1": {"devices/pci0000:40/0000:40:02.0/0000:42:00.0", "dfl-fme.1", "devices/pci0000:40/0000:40:02.0/0000:42:00.0"},
				}),
			expectedDevTreeKeys: map[string][]string{
				"region-69528db6eb31577a8c3668f9faa081f6": {"dfl-fme.0", "dfl-fme.1"},
			},
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			err := createTestDirs(dev, sysfs, tcase.devs, tcase.sysfsdirs, tcase.sysfsfiles)
			if err != nil {
				t.Fatalf("%+v", err)
			}

			plugin, err := newDevicePluginDFL(path.Join(sysfs, "class", "fpga_region"), dev, tcase.mode)

			if err != nil {
				t.Fatalf("%+v", err)
			}

			plugin.newPort = tcase.newPort

			devTree, err := plugin.scanFPGAs()
			if err != nil {
				t.Errorf("unexpected error: '%+v'", err)
			} else {
				// Validate devTree
				if len(devTree) != len(tcase.expectedDevTreeKeys) {
					t.Errorf("unexpected device tree size: %d, expected: %d", len(devTree), len(tcase.expectedDevTreeKeys))
				}
				err = validateDevTree(tcase.expectedDevTreeKeys, devTree)
				if err != nil {
					t.Errorf("device tree validation failed: %+v", err)
				}
			}

			err = os.RemoveAll(tmpdir)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
