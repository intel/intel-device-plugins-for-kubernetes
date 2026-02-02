// Copyright 2020-2026 Intel Corporation. All Rights Reserved.
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
	"path/filepath"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/pluginutils"
	"k8s.io/klog/v2"
)

const (
	sysfsDirectory              = "/sys"
	sysfsPciBusDirectory        = sysfsDirectory + "/bus/pci"
	sysfsBusPciDevicesDirectory = sysfsPciBusDirectory + "/devices"
	sysfsBusPciDriversDirectory = sysfsPciBusDirectory + "/drivers"

	vfioDriverName = "vfio-pci"
)

// Unbinds Intel GPU devices from xe and i915 drivers and binds them to vfio-pci driver.
func main() {
	var (
		denyIds  string
		allowIds string
	)

	flag.StringVar(&denyIds, "deny-ids", "", "Comma-separated list of device IDs to deny (0x1234 format)")
	flag.StringVar(&allowIds, "allow-ids", "", "Comma-separated list of device IDs to allow (0x1234 format)")

	flag.Parse()

	if err := pluginutils.ValidatePCIDeviceIDs(allowIds); err != nil {
		klog.Fatalf("allow ID validation failed: %+v", err)
	}
	if err := pluginutils.ValidatePCIDeviceIDs(denyIds); err != nil {
		klog.Fatalf("deny ID validation failed: %+v", err)
	}

	if denyIds != "" && allowIds != "" {
		klog.Fatal("cannot use both allow-ids and deny-ids options at the same time. Please use only one of them.")
	}

	if _, err := os.Stat(filepath.Join(sysfsBusPciDriversDirectory, vfioDriverName)); os.IsNotExist(err) {
		klog.Fatal("vfio-pci driver not available on this system. Please ensure that the vfio-pci kernel module is loaded.")
	}

	if _, err := os.Stat(sysfsBusPciDevicesDirectory); os.IsNotExist(err) {
		klog.Fatal("PCI bus sysfs directory does not exist")
	}

	pciDevices, err := filepath.Glob(filepath.Join(sysfsBusPciDevicesDirectory, "????:??:??.?"))
	if err != nil {
		klog.Fatalf("failed to list PCI devices: %+v", err)
	}

	success := true

	klog.Info("Binding Intel GPU devices to vfio-pci driver")

	for _, dpath := range pciDevices {
		if !pluginutils.IsCompatibleGpuDevice(dpath, allowIds, denyIds) {
			continue
		}

		if err := pluginutils.BindDeviceToDriver(dpath, sysfsBusPciDriversDirectory, vfioDriverName); err != nil {
			klog.Errorf("failed to bind device %s to vfio-pci driver: %+v", dpath, err)

			success = false
		}
	}

	if !success {
		os.Exit(1)
	}

	klog.Info("All done")
}
