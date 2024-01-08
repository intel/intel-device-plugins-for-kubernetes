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
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/pkg/errors"
	"k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	"k8s.io/utils/strings/slices"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/rm"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

// mockNotifier implements Notifier interface.
type mockNotifier struct {
	scanDone         chan bool
	i915Count        int
	xeCount          int
	i915monitorCount int
	xeMonitorCount   int
}

// Notify stops plugin Scan.
func (n *mockNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.xeCount = len(newDeviceTree[deviceTypeXe])
	n.xeMonitorCount = len(newDeviceTree[deviceTypeXe+monitorSuffix])
	n.i915Count = len(newDeviceTree[deviceTypeI915])
	n.i915monitorCount = len(newDeviceTree[deviceTypeDefault+monitorSuffix])

	n.scanDone <- true
}

type mockResourceManager struct {
	tileCount uint64
}

func (m *mockResourceManager) CreateFractionalResourceResponse(*v1beta1.AllocateRequest) (*v1beta1.AllocateResponse, error) {
	return &v1beta1.AllocateResponse{}, &dpapi.UseDefaultMethodError{}
}
func (m *mockResourceManager) SetDevInfos(rm.DeviceInfoMap) {}

func (m *mockResourceManager) GetPreferredFractionalAllocation(*v1beta1.PreferredAllocationRequest) (*v1beta1.PreferredAllocationResponse, error) {
	return &v1beta1.PreferredAllocationResponse{}, &dpapi.UseDefaultMethodError{}
}

func (m *mockResourceManager) SetTileCountPerCard(count uint64) {
	m.tileCount = count
}

type TestCaseDetails struct {
	name string
	// test-case environment
	sysfsdirs    []string
	sysfsfiles   map[string][]byte
	symlinkfiles map[string]string
	devfsdirs    []string
	// how plugin should interpret it
	options cliOptions
	// what the result should be (i915)
	expectedI915Devs     int
	expectedI915Monitors int
	// what the result should be (xe)
	expectedXeDevs     int
	expectedXeMonitors int
}

func createTestFiles(root string, tc TestCaseDetails) (string, string, error) {
	sysfs := path.Join(root, "sys")
	devfs := path.Join(root, "dev")

	for _, devfsdir := range tc.devfsdirs {
		if err := os.MkdirAll(path.Join(devfs, devfsdir), 0750); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake device directory")
		}
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

	for source, target := range tc.symlinkfiles {
		driverPath := path.Join(sysfs, target)
		symlinkPath := path.Join(sysfs, source)

		if err := os.MkdirAll(driverPath, 0750); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake driver file.")
		}

		if err := os.Symlink(driverPath, symlinkPath); err != nil {
			return "", "", errors.Wrap(err, "Failed to create fake driver symlink file.")
		}
	}

	return sysfs, devfs, nil
}

func TestNewDevicePlugin(t *testing.T) {
	if newDevicePlugin("", "", cliOptions{sharedDevNum: 2, resourceManagement: false}) == nil {
		t.Error("Failed to create plugin")
	}

	if newDevicePlugin("", "", cliOptions{sharedDevNum: 2, resourceManagement: true}) != nil {
		t.Error("Unexpectedly managed to create resource management enabled plugin inside unit tests")
	}
}

func TestGetPreferredAllocation(t *testing.T) {
	rqt := &v1beta1.PreferredAllocationRequest{
		ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"card0-4", "card0-2", "card1-1", "card2-3", "card2-4", "card2-1", "card1-0", "card1-4", "card3-4", "card1-2", "card0-1", "card2-0", "card2-2", "card1-3", "card3-0", "card3-3", "card0-3", "card0-0", "card3-1", "card3-2"},
				AllocationSize:     4,
			},
		},
	}

	rqtNotEnough := &v1beta1.PreferredAllocationRequest{
		ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"card0-1", "card0-2", "card0-3", "card1-1"},
				AllocationSize:     3,
			},
		},
	}

	rqtErr := &v1beta1.PreferredAllocationRequest{
		ContainerRequests: []*v1beta1.ContainerPreferredAllocationRequest{
			{
				AvailableDeviceIDs: []string{"card0-4", "card1-1", "card2-3", "card2-4", "card2-1"},
				AllocationSize:     6,
			},
		},
	}

	plugin := newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "none"})
	response, _ := plugin.GetPreferredAllocation(rqt)

	sort.Strings(response.ContainerResponses[0].DeviceIDs)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-4", "card1-1", "card2-3", "card3-4"}) {
		t.Error("Unexpected return value for none preferred allocation", response.ContainerResponses[0].DeviceIDs)
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "balanced"})
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-0", "card1-0", "card2-0", "card3-0"}) {
		t.Error("Unexpected return value for balanced preferred allocation", response.ContainerResponses[0].DeviceIDs)
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "packed"})
	response, _ = plugin.GetPreferredAllocation(rqt)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-0", "card0-1", "card0-2", "card0-3"}) {
		t.Error("Unexpected return value for packed preferred allocation", response.ContainerResponses[0].DeviceIDs)
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "none"})
	response, _ = plugin.GetPreferredAllocation(rqtErr)

	if response != nil {
		t.Error("Fail to handle the input error that req.AllocationSize is greater than len(req.AvailableDeviceIDs).")
	}

	plugin = newDevicePlugin("", "", cliOptions{sharedDevNum: 5, resourceManagement: false, preferredAllocationPolicy: "none"})
	response, _ = plugin.GetPreferredAllocation(rqtNotEnough)

	sort.Strings(response.ContainerResponses[0].DeviceIDs)

	if !reflect.DeepEqual(response.ContainerResponses[0].DeviceIDs, []string{"card0-1", "card0-2", "card1-1"}) {
		t.Error("Unexpected return value for none preferred allocation with too few separate devices",
			response.ContainerResponses[0].DeviceIDs)
	}
}

func TestAllocate(t *testing.T) {
	plugin := newDevicePlugin("", "", cliOptions{sharedDevNum: 2, resourceManagement: false})

	_, err := plugin.Allocate(&v1beta1.AllocateRequest{})
	if _, ok := err.(*dpapi.UseDefaultMethodError); !ok {
		t.Errorf("Unexpected return value: %+v", err)
	}

	// mock the rm
	plugin.resMan = &mockResourceManager{}

	_, err = plugin.Allocate(&v1beta1.AllocateRequest{})
	if _, ok := err.(*dpapi.UseDefaultMethodError); !ok {
		t.Errorf("Unexpected return value: %+v", err)
	}
}

func TestScan(t *testing.T) {
	tcases := []TestCaseDetails{
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
			devfsdirs: []string{
				"card0",
				"by-path/pci-0000:00:00.0-card",
				"by-path/pci-0000:00:00.0-render",
			},
			expectedI915Devs: 1,
		},
		{
			name:      "one device with xe driver",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
			symlinkfiles: map[string]string{
				"card0/device/driver": "drivers/xe",
			},
			devfsdirs: []string{
				"card0",
				"by-path/pci-0000:00:00.0-card",
				"by-path/pci-0000:00:00.0-render",
			},
			expectedXeDevs: 1,
		},
		{
			name:      "two devices with xe driver and monitoring",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64", "card1/device/drm/card1"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			symlinkfiles: map[string]string{
				"card0/device/driver": "drivers/xe",
				"card1/device/driver": "drivers/xe",
			},
			devfsdirs: []string{
				"card0",
				"by-path/pci-0000:00:00.0-card",
				"by-path/pci-0000:00:00.0-render",
				"card1",
				"by-path/pci-0000:00:01.0-card",
				"by-path/pci-0000:00:01.0-render",
			},
			options:            cliOptions{enableMonitoring: true},
			expectedXeDevs:     2,
			expectedXeMonitors: 1,
		},
		{
			name:      "two devices with xe and i915 drivers",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64", "card1/device/drm/card1"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			symlinkfiles: map[string]string{
				"card0/device/driver": "drivers/xe",
				"card1/device/driver": "drivers/i915",
			},
			devfsdirs: []string{
				"card0",
				"by-path/pci-0000:00:00.0-card",
				"by-path/pci-0000:00:00.0-render",
				"card1",
				"by-path/pci-0000:00:01.0-card",
				"by-path/pci-0000:00:01.0-render",
			},
			options:              cliOptions{enableMonitoring: true},
			expectedXeDevs:       1,
			expectedXeMonitors:   1,
			expectedI915Devs:     1,
			expectedI915Monitors: 1,
		},
		{
			name:      "sriov-1-pf-no-vfs + monitoring",
			sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":       []byte("0x8086"),
				"card0/device/sriov_numvfs": []byte("0"),
			},
			devfsdirs:            []string{"card0"},
			options:              cliOptions{enableMonitoring: true},
			expectedI915Devs:     1,
			expectedI915Monitors: 1,
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
			expectedI915Devs: 1,
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
			expectedI915Devs: 2,
		},
		{
			name: "two devices with 13 shares + monitoring",
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			devfsdirs:            []string{"card0", "card1"},
			options:              cliOptions{sharedDevNum: 13, enableMonitoring: true},
			expectedI915Devs:     26,
			expectedI915Monitors: 1,
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
			name:      "wrong vendor with 13 shares + monitoring",
			sysfsdirs: []string{"card0/device/drm/card0"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0xbeef"),
			},
			devfsdirs: []string{"card0"},
			options:   cliOptions{sharedDevNum: 13, enableMonitoring: true},
		},
		{
			name:      "no sysfs records",
			sysfsdirs: []string{"non_gpu_card"},
		},
	}

	for _, tc := range tcases {
		if tc.options.sharedDevNum == 0 {
			tc.options.sharedDevNum = 1
		}

		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp("", "test_new_device_plugin")
			if err != nil {
				t.Fatalf("can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			sysfs, devfs, err := createTestFiles(root, tc)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			plugin := newDevicePlugin(sysfs, devfs, tc.options)

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

			err = plugin.Scan(notifier)
			// Scans in GPU plugin never fail
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
			if tc.expectedI915Devs != notifier.i915Count {
				t.Errorf("Expected %d, discovered %d devices (i915)",
					tc.expectedI915Devs, notifier.i915Count)
			}
			if tc.expectedI915Monitors != notifier.i915monitorCount {
				t.Errorf("Expected %d, discovered %d monitors (i915)",
					tc.expectedI915Monitors, notifier.i915monitorCount)
			}
			if tc.expectedXeDevs != notifier.xeCount {
				t.Errorf("Expected %d, discovered %d devices (XE)",
					tc.expectedXeDevs, notifier.xeCount)
			}
			if tc.expectedXeMonitors != notifier.xeMonitorCount {
				t.Errorf("Expected %d, discovered %d monitors (XE)",
					tc.expectedXeMonitors, notifier.xeMonitorCount)
			}
		})
	}
}

func TestScanFails(t *testing.T) {
	tc := TestCaseDetails{
		name:      "xe and i915 devices with rm will fail",
		sysfsdirs: []string{"card0/device/drm/card0", "card0/device/drm/controlD64", "card1/device/drm/card1"},
		sysfsfiles: map[string][]byte{
			"card0/device/vendor": []byte("0x8086"),
			"card1/device/vendor": []byte("0x8086"),
		},
		symlinkfiles: map[string]string{
			"card0/device/driver": "drivers/xe",
			"card1/device/driver": "drivers/i915",
		},
		devfsdirs: []string{
			"card0",
			"card1",
		},
	}

	t.Run(tc.name, func(t *testing.T) {
		root, err := os.MkdirTemp("", "test_new_device_plugin")
		if err != nil {
			t.Fatalf("can't create temporary directory: %+v", err)
		}
		// dirs/files need to be removed for the next test
		defer os.RemoveAll(root)

		sysfs, devfs, err := createTestFiles(root, tc)
		if err != nil {
			t.Errorf("unexpected error: %+v", err)
		}

		plugin := newDevicePlugin(sysfs, devfs, tc.options)

		plugin.resMan = &mockResourceManager{}

		notifier := &mockNotifier{
			scanDone: plugin.scanDone,
		}

		err = plugin.Scan(notifier)
		if err == nil {
			t.Error("unexpected nil error")
		}
	})
}

func TestScanWithRmAndTiles(t *testing.T) {
	tcs := []TestCaseDetails{
		{
			name: "two tile xe devices with rm enabled - homogeneous",
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card0/device/tile0/gt0",
				"card0/device/tile1/gt1",
				"card1/device/tile0/gt0",
				"card1/device/tile1/gt1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			symlinkfiles: map[string]string{
				"card0/device/driver": "drivers/xe",
				"card1/device/driver": "drivers/xe",
			},
			devfsdirs: []string{
				"card0",
				"card1",
			},
		},
		{
			name: "2 & 1 tile xe devices with rm enabled - heterogeneous",
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card0/device/tile0/gt0",
				"card0/device/tile1/gt1",
				"card1/device/tile0/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			symlinkfiles: map[string]string{
				"card0/device/driver": "drivers/xe",
				"card1/device/driver": "drivers/xe",
			},
			devfsdirs: []string{
				"card0",
				"card1",
			},
		},
	}

	expectedTileCounts := []uint64{2, 0}

	for i, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp("", "test_new_device_plugin")
			if err != nil {
				t.Fatalf("can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			sysfs, devfs, err := createTestFiles(root, tc)
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			}

			plugin := newDevicePlugin(sysfs, devfs, tc.options)

			rm := &mockResourceManager{}
			plugin.resMan = rm

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

			err = plugin.Scan(notifier)
			if err != nil {
				t.Error("unexpected error")
			}
			if rm.tileCount != expectedTileCounts[i] {
				t.Error("unexpected tilecount for RM")
			}
		})
	}
}

// Would be nice to combine these with the overall Scan unit tests.
func createBypathTestFiles(t *testing.T, card, root, linkFile string, bypathFiles []string) (string, string) {
	drmPath := path.Join(root, "sys/class/drm/", card)
	devPath := path.Join(root, "sys", linkFile)
	byPath := path.Join(root, "by-path")

	if linkFile != "" {
		if err := os.MkdirAll(filepath.Dir(devPath), os.ModePerm); err != nil {
			t.Fatal("Couldn't create test dev dir", err)
		}

		if err := os.MkdirAll(filepath.Dir(drmPath), os.ModePerm); err != nil {
			t.Fatal("Couldn't create test drm dir", err)
		}

		if err := os.WriteFile(devPath, []byte{0}, os.ModePerm); err != nil {
			t.Fatal("Couldn't create card file", err)
		}

		if err := os.Symlink(devPath, drmPath); err != nil {
			t.Fatal("Couldn't create symlink between pci path and sysfs drm path")
		}
	}

	if len(bypathFiles) > 0 {
		if err := os.MkdirAll(byPath, os.ModePerm); err != nil {
			t.Fatal("Mkdir failed:", byPath)
		}

		for _, f := range bypathFiles {
			if err := os.WriteFile(path.Join(byPath, f), []byte{1}, os.ModePerm); err != nil {
				t.Fatal("WriteFile failed:", path.Join(byPath, f))
			}
		}
	}

	return drmPath, byPath
}

func TestBypath(t *testing.T) {
	type testData struct {
		desc        string
		linkpath    string
		bypathFiles []string
		mountCount  int
	}

	const cardName string = "card0"

	tds := []testData{
		{
			"card with two by-path files",
			"00.10.2/00.334.302/0.0.1.00/0000:0f:05.0/drm/" + cardName,
			[]string{"pci-0000:0f:05.0-card", "pci-0000:0f:05.0-render"},
			2,
		},
		{
			"different by-path files",
			"00.10.2/00.334.302/0.0.1.00/0000:ff:05.0/drm/" + cardName,
			[]string{"pci-0000:0f:05.0-card", "pci-0000:0f:05.0-render"},
			0,
		},
		{
			"invalid pci address",
			"00.10.2/00.334.302/0.0.1.00/000:ff:05.1/drm/" + cardName,
			[]string{"pci-0000:0f:05.0-card", "pci-0000:0f:05.0-render"},
			0,
		},
		{
			"symlink without card",
			"00.10.2/00.334.302/0.0.1.00/0000:0f:05.0/drm",
			[]string{"pci-0000:0f:05.0-card", "pci-0000:0f:05.0-render"},
			0,
		},
		{
			"no symlink",
			"",
			[]string{"pci-0000:0f:05.0-card", "pci-0000:0f:05.0-render"},
			0,
		},
		{
			"no by-path files",
			"00.10.2/00.334.302/0.0.1.00/0000:0f:05.0/drm/" + cardName,
			[]string{},
			0,
		},
	}

	for _, td := range tds {
		root, err := os.MkdirTemp("", "test_bypath_mounting")
		if err != nil {
			t.Fatalf("can't create temporary directory: %+v", err)
		}
		// dirs/files need to be removed for the next test
		defer os.RemoveAll(root)

		plugin := newDevicePlugin("/", "/", cliOptions{})

		drmPath, byPath := createBypathTestFiles(t, cardName, root, td.linkpath, td.bypathFiles)

		mounts := plugin.bypathMountsForPci(drmPath, cardName, byPath)

		if len(mounts) != td.mountCount {
			t.Errorf("%s: Wrong number of mounts %d vs. %d", td.desc, len(mounts), td.mountCount)
		}

		absPaths := []string{}
		for _, link := range td.bypathFiles {
			absPaths = append(absPaths, path.Join(byPath, link))
		}

		for _, mount := range mounts {
			if !slices.Contains(absPaths, mount.ContainerPath) {
				t.Errorf("%s: containerpath is incorrect: %s", td.desc, mount.ContainerPath)
			}

			if !slices.Contains(absPaths, mount.HostPath) {
				t.Errorf("%s: hostpath is incorrect: %s", td.desc, mount.HostPath)
			}
		}
	}
}
