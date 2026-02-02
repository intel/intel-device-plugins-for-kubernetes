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

package pluginutils

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const (
	vendorId      = "8086"          // Intel vendor ID
	vendorString  = "0x" + vendorId // Intel vendor ID
	vfioPciDriver = "vfio-pci"
)

func DeviceDriverName(devicePath, defaultDriver string) string {
	driverName := defaultDriver

	linkpath, err := os.Readlink(filepath.Join(devicePath, "driver"))
	if err == nil {
		driverName = filepath.Base(linkpath)
	}

	return driverName
}

// IsCompatibleGpuDevice checks if the PCI device at dpath is a compatible GPU device
// The dpath may be a drm device (/sys/class/drm/cardX/device) or a
// pci device path (/sys/bus/pci/devices/0000:00:02.0).
func IsCompatibleGpuDevice(dpath, allowIds, denyIds string) bool {
	dat, err := os.ReadFile(path.Join(dpath, "vendor"))
	if err != nil {
		klog.Warning("Skipping. Can't read vendor file: ", err)
		return false
	}

	if strings.TrimSpace(string(dat)) != vendorString {
		klog.V(4).Info("Non-Intel device: ", dpath)
		return false
	}

	dat, err = os.ReadFile(path.Join(dpath, "class"))
	if err != nil {
		klog.Warning("Skipping. Can't read class file: ", err)
		return false
	}

	// Check for vga/display controller class (0x0300 or 0x0380)
	if !(strings.HasPrefix(strings.TrimSpace(string(dat)), "0x0300") ||
		strings.HasPrefix(strings.TrimSpace(string(dat)), "0x0380")) {
		klog.V(4).Info("Not a display controller device: ", dpath)
		return false
	}

	dat, err = os.ReadFile(path.Join(dpath, "device"))
	if err != nil {
		klog.Warning("Skipping. Can't read device file: ", err)
		return false
	}

	deviceId := strings.TrimSpace(string(dat))
	if len(denyIds) > 0 && strings.Contains(denyIds, deviceId) {
		klog.V(4).Infof("Skipping device %s, in denylist: %s", dpath, denyIds)
		return false
	}
	if len(allowIds) > 0 && !strings.Contains(allowIds, deviceId) {
		klog.V(4).Infof("Skipping device %s, not in allowlist: %s", dpath, allowIds)
		return false
	}

	dat, err = os.ReadFile(path.Join(dpath, "sriov_numvfs"))
	if err == nil && strings.TrimSpace(string(dat)) != "0" {
		klog.V(4).Infof("Skipping device %s, SR-IOV PF device with VFs", dpath)
		return false
	}

	return true
}

func IsCompatibleGpuVfioDevice(dpath, allowIds, denyIds string) bool {
	if !IsCompatibleGpuDevice(dpath, allowIds, denyIds) {
		return false
	}

	driver := DeviceDriverName(dpath, "")
	if driver != vfioPciDriver {
		klog.V(4).Infof("Skipping device %s, incorrect driver: %s", dpath, driver)
		return false
	}

	klog.V(3).Info("Display controller device: ", dpath)

	return true
}

func ValidatePCIDeviceIDs(pciIDList string) error {
	if pciIDList == "" {
		return nil
	}

	r := regexp.MustCompile(`^0x[0-9a-f]{4}$`)

	for id := range strings.SplitSeq(pciIDList, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			return os.ErrNotExist
		}

		if !r.MatchString(id) {
			return os.ErrInvalid
		}
	}

	return nil
}

func BindDeviceToDriver(devicePath, driversPath, driverName string) error {
	bdfAddress := filepath.Base(devicePath)

	klog.Info("Trying to bind device ", bdfAddress, " to driver ", driverName)

	// Unbind from current driver if any
	currentDriverLink := filepath.Join(devicePath, "driver")
	if _, err := os.Lstat(currentDriverLink); err == nil {
		currentDriverPath, err := os.Readlink(currentDriverLink)
		if err != nil {
			return err
		}

		driverNow := filepath.Base(currentDriverPath)
		if driverNow == driverName {
			// Already bound to the desired driver
			klog.Infof("Device %s is already bound to driver %s", bdfAddress, driverName)
			return nil
		}

		currentDriverUnbindPath := filepath.Join(currentDriverLink, "unbind")

		klog.Infof("Unbinding device %s from current driver: %s", bdfAddress, currentDriverUnbindPath)

		if err := os.WriteFile(currentDriverUnbindPath, []byte(bdfAddress), 0200); err != nil {
			return err
		}
	}

	// Read device ID to write to "new_id"
	deviceIDPath := filepath.Join(devicePath, "device")
	deviceID, err := os.ReadFile(deviceIDPath)
	if err != nil {
		return err
	}

	deviceIdStr := strings.TrimPrefix(strings.TrimSpace(string(deviceID)), "0x")
	newIDPath := filepath.Join(driversPath, driverName, "new_id")
	newIdData := fmt.Appendf(nil, "%s %s", vendorId, deviceIdStr)

	klog.Info("Writing new_id for driver ", driverName, ": ", string(newIdData))

	if err := os.WriteFile(newIDPath, newIdData, 0200); err != nil {
		return err
	}

	klog.Info("Waiting for device ", bdfAddress, " to bind to driver ", driverName)

	// Up to 2 seconds of wait for the driver to bind
	for range 20 {
		dn := DeviceDriverName(devicePath, "unknown")
		if dn == driverName {
			goto End
		}

		// Small delay to allow the driver to bind
		time.Sleep(100 * time.Millisecond)
	}

	klog.Info("Failed to bind device ", bdfAddress, " to driver ", driverName, " using new_id method, trying bind method")

	// Try to bind the device to the driver if the new_id method didn't work
	if err := os.WriteFile(filepath.Join(driversPath, driverName, "bind"), []byte(bdfAddress), 0200); err != nil {
		return err
	}

End:
	klog.Infof("Bound device %s to driver %s", bdfAddress, driverName)

	return nil
}
