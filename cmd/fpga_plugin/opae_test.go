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
	"os"
	"path"
	"reflect"
	"testing"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func TestNewDevicePluginOPAE(t *testing.T) {
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
			_, err := newDevicePluginOPAE("", "", tcase.mode)
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
func getDevicesOPAE() []device {
	return []device{
		{
			name: "intel-fpga-dev.0",
			regions: []region{
				{
					id:          "intel-fpga-fme.0",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/intel-fpga-fme.0",
					afus: []afu{
						{
							id:      "intel-fpga-port.0",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/intel-fpga-port.0",
						},
					},
				},
			},
		},
		{
			name: "intel-fpga-dev.1",
			regions: []region{
				{
					id:          "intel-fpga-fme.1",
					interfaceID: "ce48969398f05f33946d560708be108a",
					devNode:     "/dev/intel-fpga-fme.1",
					afus: []afu{
						{
							id:      "intel-fpga-port.1",
							afuID:   "d8424dc4a4a3c413f89e433683f9040b",
							devNode: "/dev/intel-fpga-port.1",
						},
					},
				},
			},
		},
		{
			name: "intel-fpga-dev.2",
			regions: []region{
				{
					id:          "intel-fpga-fme.2",
					interfaceID: unhealthyInterfaceID,
					devNode:     "/dev/intel-fpga-fme.2",
					afus: []afu{
						{
							id:      "intel-fpga-port.2",
							afuID:   unhealthyAfuID,
							devNode: "/dev/intel-fpga-port.2",
						},
					},
				},
			},
		},
	}
}

func TestGetRegionDevelTreeOPAE(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	nodes := []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.0",
			ContainerPath: "/dev/intel-fpga-port.0",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/intel-fpga-fme.0",
			ContainerPath: "/dev/intel-fpga-fme.0",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.0", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.1",
			ContainerPath: "/dev/intel-fpga-port.1",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/intel-fpga-fme.1",
			ContainerPath: "/dev/intel-fpga-fme.1",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.1", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.2",
			ContainerPath: "/dev/intel-fpga-port.2",
			Permissions:   "rw",
		},
		{
			HostPath:      "/dev/intel-fpga-fme.2",
			ContainerPath: "/dev/intel-fpga-fme.2",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "intel-fpga-fme.2", dpapi.NewDeviceInfo(pluginapi.Unhealthy, nodes, nil, nil, nil))

	result := getRegionDevelTree(getDevicesOPAE())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetRegionTreeOPAE(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	nodes := []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.0",
			ContainerPath: "/dev/intel-fpga-port.0",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.0", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.1",
			ContainerPath: "/dev/intel-fpga-port.1",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-ce48969398f05f33946d560708be108a", "intel-fpga-fme.1", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.2",
			ContainerPath: "/dev/intel-fpga-port.2",
			Permissions:   "rw",
		},
	}
	expected.AddDevice(regionMode+"-"+unhealthyInterfaceID, "intel-fpga-fme.2", dpapi.NewDeviceInfo(pluginapi.Unhealthy, nodes, nil, nil, nil))

	result := getRegionTree(getDevicesOPAE())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

func TestGetAfuTreeOPAE(t *testing.T) {
	expected := dpapi.NewDeviceTree()
	nodes := []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.0",
			ContainerPath: "/dev/intel-fpga-port.0",
			Permissions:   "rw",
		},
	}
	expected.AddDevice("af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs", "intel-fpga-port.0", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.1",
			ContainerPath: "/dev/intel-fpga-port.1",
			Permissions:   "rw",
		},
	}
	expected.AddDevice("af-ce4.d84.zkiWk5jwXzOUbVYHCL4QithCTcSko8QT-J5DNoP5BAs", "intel-fpga-port.1", dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil))

	nodes = []pluginapi.DeviceSpec{
		{
			HostPath:      "/dev/intel-fpga-port.2",
			ContainerPath: "/dev/intel-fpga-port.2",
			Permissions:   "rw",
		},
	}
	expected.AddDevice("af-fff.fff.__________________________________________8", "intel-fpga-port.2", dpapi.NewDeviceInfo(pluginapi.Unhealthy, nodes, nil, nil, nil))

	result := getAfuTree(getDevicesOPAE())
	if !reflect.DeepEqual(result, expected) {
		t.Error("Got unexpected result: ", result)
	}
}

// genNewIntelFpgaPort generates newPortFunc function.
func genNewIntelFpgaPort(sysFsPrefix, devDir string, sysFsInfo map[string][]string) newPortFunc {
	return func(portName string) (fpga.Port, error) {
		portPciDev := sysFsInfo[portName][0]
		portPciDevSysFs := path.Join(sysFsPrefix, portPciDev)

		fmeName := sysFsInfo[portName][1]
		fmePciDev := sysFsInfo[portName][2]
		fmePciDevSysFs := path.Join(sysFsPrefix, fmePciDev)

		return &fpga.IntelFpgaPort{
			Name:      portName,
			DevPath:   path.Join(devDir, portName),
			SysFsPath: path.Join(portPciDevSysFs, "fpga", "intel-fpga-dev*", portName),
			FME: &fpga.IntelFpgaFME{
				Name:      fmeName,
				DevPath:   path.Join(devDir, fmeName),
				SysFsPath: fmePciDevSysFs,
				PCIDevice: &fpga.PCIDevice{SysFsPath: fmePciDevSysFs},
			},
		}, nil
	}
}

func TestScanFPGAsOPAE(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "TestScanFPGAsOPAE")
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
			name: "Valid OPAE scan in af mode",
			mode: afMode,
			devs: []string{
				"intel-fpga-fme.0", "intel-fpga-port.0",
				"intel-fpga-fme.1", "intel-fpga-port.1",
			},
			sysfsdirs: []string{
				"class/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"class/fpga/intel-fpga-dev.1/intel-fpga-port.1",
				"class/fpga/dir", // this should be skipped by plugin.ScanFPGAs
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-port.1/",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr",
			},
			sysfsfiles: map[string][]byte{
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-port.1/afu_id": []byte("f7df405cbd7acf7222f144b0b93acd18\n"),

				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			newPort: genNewIntelFpgaPort(sysfs, dev,
				map[string][]string{
					"intel-fpga-port.0": {"devices/pci0000:00/0000:00:03.2/0000:06:00.0", "intel-fpga-fme.0", "devices/pci0000:00/0000:00:03.2/0000:06:00.0"},
					"intel-fpga-port.1": {"devices/pci0000:40/0000:40:02.0/0000:42:00.0", "intel-fpga-fme.1", "devices/pci0000:40/0000:40:02.0/0000:42:00.0"},
				}),
			expectedDevTreeKeys: map[string][]string{
				"af-695.d84.aVKNtusxV3qMNmj5-qCB9thCTcSko8QT-J5DNoP5BAs": {"intel-fpga-port.0"},
				"af-695.f7d.aVKNtusxV3qMNmj5-qCB9vffQFy9es9yIvFEsLk6zRg": {"intel-fpga-port.1"},
			},
		},
		{
			name: "Valid OPAE scan in af mode (SR-IOV)",
			mode: afMode,
			devs: []string{
				"intel-fpga-fme.0", "intel-fpga-port.0",
				"intel-fpga-fme.1", "intel-fpga-port.2",
			},
			sysfsdirs: []string{
				"class/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"class/fpga/intel-fpga-dev.1/intel-fpga-fme.1",
				"class/fpga/intel-fpga-dev.2/intel-fpga-port.2",
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.1/fpga/intel-fpga-dev.2/intel-fpga-port.2",
			},
			sysfsfiles: map[string][]byte{
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.1/fpga/intel-fpga-dev.2/intel-fpga-port.2/afu_id": []byte("f7df405cbd7acf7222f144b0b93acd18\n"),

				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			newPort: genNewIntelFpgaPort(sysfs, dev,
				map[string][]string{
					"intel-fpga-port.0": {"devices/pci0000:00/0000:00:03.2/0000:06:00.0", "intel-fpga-fme.0", "devices/pci0000:00/0000:00:03.2/0000:06:00.0"},
					"intel-fpga-port.2": {"devices/pci0000:40/0000:40:02.0/0000:42:00.1", "intel-fpga-fme.1", "devices/pci0000:40/0000:40:02.0/0000:42:00.0"},
				}),
			expectedDevTreeKeys: map[string][]string{
				"af-695.d84.aVKNtusxV3qMNmj5-qCB9thCTcSko8QT-J5DNoP5BAs": {"intel-fpga-port.0"},
				"af-695.f7d.aVKNtusxV3qMNmj5-qCB9vffQFy9es9yIvFEsLk6zRg": {"intel-fpga-port.2"},
			},
		},
		{
			name: "Valid OPAE scan in region mode (SR-IOV)",
			mode: regionMode,
			devs: []string{
				"intel-fpga-fme.0", "intel-fpga-port.0",
				"intel-fpga-fme.1", "intel-fpga-port.2",
			},
			sysfsdirs: []string{
				"class/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"class/fpga/intel-fpga-dev.1/intel-fpga-fme.1",
				"class/fpga/intel-fpga-dev.2/intel-fpga-port.2",
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0",
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr",
				"devices/pci0000:40/0000:40:02.0/0000:42:00.1/fpga/intel-fpga-dev.2/intel-fpga-port.2",
			},
			sysfsfiles: map[string][]byte{
				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.1/fpga/intel-fpga-dev.2/intel-fpga-port.2/afu_id": []byte("f7df405cbd7acf7222f144b0b93acd18\n"),

				"devices/pci0000:00/0000:00:03.2/0000:06:00.0/fpga/intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
				"devices/pci0000:40/0000:40:02.0/0000:42:00.0/fpga/intel-fpga-dev.1/intel-fpga-fme.1/pr/interface_id": []byte("69528db6eb31577a8c3668f9faa081f6\n"),
			},
			newPort: genNewIntelFpgaPort(sysfs, dev,
				map[string][]string{
					"intel-fpga-port.0": {"devices/pci0000:00/0000:00:03.2/0000:06:00.0", "intel-fpga-fme.0", "devices/pci0000:00/0000:00:03.2/0000:06:00.0"},
					"intel-fpga-port.2": {"devices/pci0000:40/0000:40:02.0/0000:42:00.1", "intel-fpga-fme.1", "devices/pci0000:40/0000:40:02.0/0000:42:00.0"},
				}),
			expectedDevTreeKeys: map[string][]string{
				"region-69528db6eb31577a8c3668f9faa081f6": {"intel-fpga-fme.0", "intel-fpga-fme.1"},
			},
		},

		{
			name: "No sysfs device entries",
			mode: afMode,
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			err := createTestDirs(dev, sysfs, tcase.devs, tcase.sysfsdirs, tcase.sysfsfiles)
			if err != nil {
				t.Fatalf("%+v", err)
			}

			plugin, err := newDevicePluginOPAE(path.Join(sysfs, "class", "fpga"), dev, tcase.mode)

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
