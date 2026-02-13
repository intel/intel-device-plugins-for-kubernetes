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

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/pkg/errors"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	vendorId      = "8086"          // Intel vendor ID
	vendorString  = "0x" + vendorId // Intel vendor ID
	vfioPciDriver = "vfio-pci"
	vfioPath      = "/dev/vfio"
	vfioCtrlPath  = "/dev/vfio/vfio"

	VfioBdfPrefix = "VFIO_BDF"

	unknownDriver = "unknown"
)

type DeviceIdValidationError struct {
	reason string
}

func (e DeviceIdValidationError) Error() string {
	return fmt.Sprintf("PCI device ID validation error: %s", e.reason)
}

type DeviceBindingError struct {
	reason string
}

func (e DeviceBindingError) Error() string {
	return fmt.Sprintf("device binding error: %s", e.reason)
}

type filterPciDeviceFunc func(dpath string) (bool, error)

func DeviceDriverName(devicePath, defaultDriver string) string {
	driverName := defaultDriver

	linkpath, err := os.Readlink(filepath.Join(devicePath, "driver"))
	if err == nil {
		driverName = filepath.Base(linkpath)
	}

	return driverName
}

func deviceIdsToMap(deviceIdList string) map[string]struct{} {
	devIdMap := make(map[string]struct{})

	for id := range strings.SplitSeq(deviceIdList, ",") {
		id = strings.ToLower(strings.TrimSpace(id))
		if id != "" {
			devIdMap[id] = struct{}{}
		}
	}

	return devIdMap
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
	if len(denyIds) > 0 {
		if _, found := deviceIdsToMap(denyIds)[deviceId]; found {
			klog.V(4).Infof("Skipping device %s, in denylist: %s", dpath, denyIds)
			return false
		}
	}
	if len(allowIds) > 0 {
		if _, found := deviceIdsToMap(allowIds)[deviceId]; !found {
			klog.V(4).Infof("Skipping device %s, not in allowlist: %s", dpath, allowIds)
			return false
		}
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

// ValidatePCIDeviceIDs validates that the provided comma-separated list of PCI
// device IDs is in the correct format (0x followed by 4 hexadecimal digits).
func ValidatePCIDeviceIDs(pciIDList string) error {
	if pciIDList == "" {
		return nil
	}

	r := regexp.MustCompile(`^0x[0-9a-fA-F]{4}$`)

	for id := range strings.SplitSeq(pciIDList, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			return DeviceIdValidationError{reason: "empty PCI device ID"}
		}

		if !r.MatchString(id) {
			return DeviceIdValidationError{reason: fmt.Sprintf("invalid PCI device ID (%s)", id)}
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

	var dn string

	// Up to 2 seconds of wait for the driver to bind
	for range 20 {
		dn = DeviceDriverName(devicePath, unknownDriver)
		if dn == driverName {
			goto End
		}

		// Small delay to allow the driver to bind
		time.Sleep(100 * time.Millisecond)
	}

	klog.Info("Failed to bind device ", bdfAddress, " to driver ", driverName, " with new_id, trying bind...")

	// Try to bind the device to the driver if the new_id method didn't work
	if err := os.WriteFile(filepath.Join(driversPath, driverName, "bind"), []byte(bdfAddress), 0200); err != nil {
		return err
	}

	dn = DeviceDriverName(devicePath, unknownDriver)
	if dn != driverName {
		return DeviceBindingError{
			reason: fmt.Sprintf("failed to bind %s to driver %s, current driver: %s", bdfAddress, driverName, dn),
		}
	}

End:
	klog.Infof("Bound device %s to driver %s", bdfAddress, driverName)

	return nil
}

// scan collects devices by scanning sysfs and devfs entries.
func PciScan(filterFunc filterPciDeviceFunc, devDir string) (dpapi.DeviceTree, error) {
	// scan sysfs tree
	pciDevices, err := filepath.Glob(filepath.Join(devDir, "????:??:??.?"))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	devTree := dpapi.NewDeviceTree()
	devNum := 0

	for _, dpath := range pciDevices {
		if ok, err := filterFunc(dpath); err != nil {
			return nil, err
		} else if !ok {
			continue
		}

		// device belongs to an IOMMU group
		iommu_group, err := filepath.EvalSymlinks(filepath.Join(dpath, "iommu_group"))
		if err != nil {
			return nil, errors.WithStack(err)
		}

		vfioDevPath := filepath.Join(vfioPath, filepath.Base(iommu_group))

		devNodes := []pluginapi.DeviceSpec{
			{
				HostPath:      vfioDevPath,
				ContainerPath: vfioDevPath,
				Permissions:   "rw",
			},
			{
				HostPath:      vfioCtrlPath,
				ContainerPath: vfioCtrlPath,
				Permissions:   "rw",
			},
		}

		// TODO: add IOMMUFD nodes
		// iommuFdDevices, err := filepath.Glob(filepath.Join(dpath, "vfio-dev", "vfio?"))
		// if err == nil {
		// 	for _, iommuDev := range iommuFdDevices {
		// 		devNodes = append(devNodes, pluginapi.DeviceSpec{
		// 			HostPath:      filepath.Join(vfioPath, "devices", filepath.Base(iommuDev)),
		// 			ContainerPath: filepath.Join(vfioPath, "devices", filepath.Base(iommuDev)),
		// 			Permissions:   "rw",
		// 		})
		// 	}
		// }

		bdf := filepath.Base(dpath)

		envs := map[string]string{
			fmt.Sprintf("%s%d", VfioBdfPrefix, devNum): bdf,
		}

		klog.V(4).Infof("%s: nodes: %+v", bdf, devNodes)
		devTree.AddDevice("vfio", bdf, dpapi.NewDeviceInfo(pluginapi.Healthy, devNodes, nil, envs, nil, nil))

		devNum = devNum + 1
	}

	return devTree, nil
}
