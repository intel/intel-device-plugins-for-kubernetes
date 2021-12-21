// Copyright 2017-2021 Intel Corporation. All Rights Reserved.
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

// Package dpdkdrv implements QAT device plugin for DPDK driver.
package dpdkdrv

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

const (
	uioDevicePath      = "/dev"
	vfioDevicePath     = "/dev/vfio"
	vfioCtrlDevicePath = vfioDevicePath + "/vfio"
	uioMountPath       = "/sys/class/uio"
	pciDeviceDirectory = "/sys/bus/pci/devices"
	pciDriverDirectory = "/sys/bus/pci/drivers"
	uioSuffix          = "uio"
	iommuGroupSuffix   = "iommu_group"
	vendorPrefix       = "8086 "
	envVarPrefix       = "QAT"

	igbUio  = "igb_uio"
	vfioPci = "vfio-pci"

	// Period of device scans.
	scanPeriod = 5 * time.Second
)

// QAT PCI VF Device ID -> kernel QAT VF device driver mappings.
var qatDeviceDriver = map[string]string{
	"0442": "dh895xccvf",
	"0443": "dh895xccvf",
	"18a1": "c4xxxvf",
	"19e3": "c3xxxvf",
	"4941": "4xxxvf",
	"37c9": "c6xxvf",
	"6f55": "d15xxvf",
}

// DevicePlugin represents vfio based QAT plugin.
type DevicePlugin struct {
	scanTicker *time.Ticker
	scanDone   chan bool

	pciDriverDir    string
	pciDeviceDir    string
	dpdkDriver      string
	kernelVfDrivers []string
	maxDevices      int
}

// NewDevicePlugin returns new instance of vfio based QAT plugin.
func NewDevicePlugin(maxDevices int, kernelVfDrivers string, dpdkDriver string) (*DevicePlugin, error) {
	if !isValidDpdkDeviceDriver(dpdkDriver) {
		return nil, errors.Errorf("wrong DPDK device driver: %s", dpdkDriver)
	}

	kernelDrivers := strings.Split(kernelVfDrivers, ",")
	for _, driver := range kernelDrivers {
		if !isValidKernelDriver(driver) {
			return nil, errors.Errorf("wrong kernel VF driver: %s", driver)
		}
	}

	return newDevicePlugin(pciDriverDirectory, pciDeviceDirectory, maxDevices, kernelDrivers, dpdkDriver), nil
}

func newDevicePlugin(pciDriverDir, pciDeviceDir string, maxDevices int, kernelVfDrivers []string, dpdkDriver string) *DevicePlugin {
	return &DevicePlugin{
		maxDevices:      maxDevices,
		pciDriverDir:    pciDriverDir,
		pciDeviceDir:    pciDeviceDir,
		kernelVfDrivers: kernelVfDrivers,
		dpdkDriver:      dpdkDriver,
		scanTicker:      time.NewTicker(scanPeriod),
		scanDone:        make(chan bool, 1),
	}
}

func (dp *DevicePlugin) setupDeviceIDs() error {
	for devID, driver := range qatDeviceDriver {
		for _, enabledDriver := range dp.kernelVfDrivers {
			if driver != enabledDriver {
				continue
			}

			err := writeToDriver(filepath.Join(dp.pciDriverDir, dp.dpdkDriver, "new_id"), vendorPrefix+devID)
			if err != nil && !errors.Is(err, os.ErrExist) {
				return errors.WithMessagef(err, "failed to set device ID %s for %s. Driver module not loaded?", devID, dp.dpdkDriver)
			}
		}
	}

	return nil
}

// Scan implements Scanner interface for vfio based QAT plugin.
func (dp *DevicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	if err := dp.setupDeviceIDs(); err != nil {
		return err
	}

	for {
		devTree, err := dp.scan()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		select {
		case <-dp.scanDone:
			return nil
		case <-dp.scanTicker.C:
		}
	}
}

func (dp *DevicePlugin) getDpdkDevice(vfBdf string) (string, error) {
	switch dp.dpdkDriver {
	case igbUio:
		uioDirPath := filepath.Join(dp.pciDeviceDir, vfBdf, uioSuffix)

		files, err := os.ReadDir(uioDirPath)
		if err != nil {
			return "", err
		}

		if len(files) == 0 {
			return "", errors.New("No devices found")
		}

		return files[0].Name(), nil

	case vfioPci:
		vfioDirPath := filepath.Join(dp.pciDeviceDir, vfBdf, iommuGroupSuffix)
		group, err := filepath.EvalSymlinks(vfioDirPath)

		if err != nil {
			return "", errors.WithStack(err)
		}

		s := filepath.Base(group)

		return s, nil

	default:
		return "", errors.New("Unknown DPDK driver")
	}
}

func (dp *DevicePlugin) getDpdkDeviceSpecs(dpdkDeviceName string) []pluginapi.DeviceSpec {
	switch dp.dpdkDriver {
	case igbUio:
		//Setting up with uio
		uioDev := filepath.Join(uioDevicePath, dpdkDeviceName)

		return []pluginapi.DeviceSpec{
			{
				HostPath:      uioDev,
				ContainerPath: uioDev,
				Permissions:   "rw",
			},
		}
	case vfioPci:
		//Setting up with vfio
		vfioDev := filepath.Join(vfioDevicePath, dpdkDeviceName)

		return []pluginapi.DeviceSpec{
			{
				HostPath:      vfioDev,
				ContainerPath: vfioDev,
				Permissions:   "rw",
			},
			{
				HostPath:      vfioCtrlDevicePath,
				ContainerPath: vfioCtrlDevicePath,
				Permissions:   "rw",
			},
		}
	default:
		return nil
	}
}

func (dp *DevicePlugin) getDpdkMounts(dpdkDeviceName string) []pluginapi.Mount {
	switch dp.dpdkDriver {
	case igbUio:
		//Setting up with uio mountpoints
		uioMountPoint := filepath.Join(uioMountPath, dpdkDeviceName, "/device")

		return []pluginapi.Mount{
			{
				HostPath:      uioMountPoint,
				ContainerPath: uioMountPoint,
			},
		}
	case vfioPci:
		//No mountpoint for vfio needs to be populated
		return nil
	default:
		return nil
	}
}

func getDeviceID(device string) (string, error) {
	devID, err := os.ReadFile(filepath.Join(device, "device"))
	if err != nil {
		return "", errors.Wrapf(err, "failed to read device ID")
	}

	return strings.TrimPrefix(string(bytes.TrimSpace(devID)), "0x"), nil
}

func writeToDriver(path, value string) error {
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		return errors.Wrapf(err, "write to driver failed: %s", value)
	}

	return nil
}

func isValidKernelDriver(kernelvfDriver string) bool {
	for _, driver := range qatDeviceDriver {
		if driver == kernelvfDriver {
			return true
		}
	}

	return false
}

func isValidDpdkDeviceDriver(dpdkDriver string) bool {
	switch dpdkDriver {
	case igbUio, vfioPci:
		return true
	}

	return false
}

func (dp *DevicePlugin) isValidVfDeviceID(vfDevID string) bool {
	if driver, ok := qatDeviceDriver[vfDevID]; ok {
		for _, enabledDriver := range dp.kernelVfDrivers {
			if driver == enabledDriver {
				return true
			}
		}
	}

	klog.Warningf("device ID %s is not a QAT device or not enabled by kernelVfDrivers.", vfDevID)

	return false
}

// PostAllocate implements PostAllocator interface for vfio based QAT plugin.
func (dp *DevicePlugin) PostAllocate(response *pluginapi.AllocateResponse) error {
	tempMap := make(map[string]string)

	for _, cresp := range response.ContainerResponses {
		counter := 0

		for k := range cresp.Envs {
			tempMap[strings.Join([]string{envVarPrefix, strconv.Itoa(counter)}, "")] = cresp.Envs[k]
			counter++
		}

		cresp.Envs = tempMap
	}

	return nil
}

func getPciDevicesWithPattern(pattern string) (pciDevices []string) {
	pciDevices = make([]string, 0)

	devs, err := filepath.Glob(pattern)
	if err != nil {
		klog.Warningf("bad pattern: %s", pattern)
		return
	}

	for _, devBdf := range devs {
		targetDev, err := filepath.EvalSymlinks(devBdf)
		if err != nil {
			klog.Warningf("unable to evaluate symlink: %s", devBdf)
			continue
		}

		pciDevices = append(pciDevices, targetDev)
	}

	return
}

func (dp *DevicePlugin) getVfDevices() []string {
	qatPfDevices := make([]string, 0)
	qatVfDevices := make([]string, 0)

	// Get PF BDFs bound to a known QAT PF driver
	for _, vfDriver := range dp.kernelVfDrivers {
		pfDriver := strings.TrimSuffix(vfDriver, "vf")
		pattern := filepath.Join(dp.pciDriverDir, pfDriver, "????:??:??.?")
		qatPfDevices = append(qatPfDevices, getPciDevicesWithPattern(pattern)...)
	}

	// Get VF devices belonging to a valid QAT PF device
	for _, qatPfDevice := range qatPfDevices {
		pattern := filepath.Join(qatPfDevice, "virtfn*")
		qatVfDevices = append(qatVfDevices, getPciDevicesWithPattern(pattern)...)
	}

	if len(qatPfDevices) > 0 {
		if len(qatVfDevices) >= dp.maxDevices {
			return qatVfDevices[:dp.maxDevices]
		}

		return qatVfDevices
	}

	// No PF devices with a QAT driver found, running in a VM?
	pattern := filepath.Join(dp.pciDeviceDir, "????:??:??.?")
	for _, pciDev := range getPciDevicesWithPattern(pattern) {
		devID, err := getDeviceID(pciDev)
		if err != nil {
			klog.Warningf("unable to read device id for device %s: %q", filepath.Base(pciDev), err)
			continue
		}

		if dp.isValidVfDeviceID(devID) {
			qatVfDevices = append(qatVfDevices, pciDev)
		}
	}

	if len(qatVfDevices) >= dp.maxDevices {
		return qatVfDevices[:dp.maxDevices]
	}

	return qatVfDevices
}

func getCurrentDriver(device string) string {
	symlink := filepath.Join(device, "driver")

	driver, err := filepath.EvalSymlinks(symlink)
	if err != nil {
		klog.Infof("no driver bound to device %q", filepath.Base(device))
		return ""
	}

	return filepath.Base(driver)
}

func (dp *DevicePlugin) scan() (dpapi.DeviceTree, error) {
	devTree := dpapi.NewDeviceTree()
	n := 0

	for _, vfDevice := range dp.getVfDevices() {
		vfBdf := filepath.Base(vfDevice)

		if drv := getCurrentDriver(vfDevice); drv != dp.dpdkDriver {
			if drv != "" {
				err := writeToDriver(filepath.Join(dp.pciDriverDir, drv, "unbind"), vfBdf)
				if err != nil {
					return nil, err
				}
			}

			err := writeToDriver(filepath.Join(dp.pciDriverDir, dp.dpdkDriver, "bind"), vfBdf)
			if err != nil {
				return nil, err
			}
		}

		dpdkDeviceName, err := dp.getDpdkDevice(vfBdf)
		if err != nil {
			return nil, err
		}

		klog.V(1).Infof("Device %s found", vfBdf)

		n = n + 1
		envs := map[string]string{
			fmt.Sprintf("%s%d", envVarPrefix, n): vfBdf,
		}

		devinfo := dpapi.NewDeviceInfo(pluginapi.Healthy, dp.getDpdkDeviceSpecs(dpdkDeviceName), dp.getDpdkMounts(dpdkDeviceName), envs)

		devTree.AddDevice("generic", vfBdf, devinfo)
	}

	return devTree, nil
}
