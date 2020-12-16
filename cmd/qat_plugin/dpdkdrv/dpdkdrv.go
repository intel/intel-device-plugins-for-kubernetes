// Copyright 2017 Intel Corporation. All Rights Reserved.
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
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog"
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
	newIDSuffix        = "new_id"
	driverUnbindSuffix = "driver/unbind"
	vendorPrefix       = "8086 "
	envVarPrefix       = "QAT"

	igbUio  = "igb_uio"
	vfioPci = "vfio-pci"
)

// DevicePlugin represents vfio based QAT plugin.
type DevicePlugin struct {
	maxDevices      int
	pciDriverDir    string
	pciDeviceDir    string
	kernelVfDrivers []string
	dpdkDriver      string
}

// NewDevicePlugin returns new instance of vfio based QAT plugin.
func NewDevicePlugin(maxDevices int, kernelVfDrivers string, dpdkDriver string) (*DevicePlugin, error) {
	if !isValidDpdkDeviceDriver(dpdkDriver) {
		return nil, errors.Errorf("wrong DPDK device driver: %s", dpdkDriver)
	}

	kernelDrivers := strings.Split(kernelVfDrivers, ",")
	for _, driver := range kernelDrivers {
		if !isValidKerneDriver(driver) {
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
	}
}

// Scan implements Scanner interface for vfio based QAT plugin.
func (dp *DevicePlugin) Scan(notifier dpapi.Notifier) error {
	for {
		devTree, err := dp.scan()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		time.Sleep(5 * time.Second)
	}
}

func (dp *DevicePlugin) getDpdkDevice(vfBdf string) (string, error) {
	switch dp.dpdkDriver {
	case igbUio:
		uioDirPath := filepath.Join(dp.pciDeviceDir, vfBdf, uioSuffix)
		files, err := ioutil.ReadDir(uioDirPath)
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
		klog.V(1).Infof("The vfio device group detected is %v", s)
		return s, nil
	}

	return "", errors.New("Unknown DPDK driver")
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

func (dp *DevicePlugin) getDeviceID(pciAddr string) (string, error) {
	devID, err := ioutil.ReadFile(filepath.Join(dp.pciDeviceDir, filepath.Clean(pciAddr), "device"))
	if err != nil {
		return "", errors.Wrapf(err, "Cannot obtain ID for the device %s", pciAddr)
	}

	return strings.TrimPrefix(string(bytes.TrimSpace(devID)), "0x"), nil
}

// bindDevice unbinds given device from kernel driver and binds to DPDK driver.
func (dp *DevicePlugin) bindDevice(vfBdf string) error {
	unbindDevicePath := filepath.Join(dp.pciDeviceDir, vfBdf, driverUnbindSuffix)

	// Unbind from the kernel driver
	err := ioutil.WriteFile(unbindDevicePath, []byte(vfBdf), 0600)
	if err != nil {
		return errors.Wrapf(err, "Unbinding from kernel driver failed for the device %s", vfBdf)
	}
	vfdevID, err := dp.getDeviceID(vfBdf)
	if err != nil {
		return err
	}
	bindDevicePath := filepath.Join(dp.pciDriverDir, dp.dpdkDriver, newIDSuffix)
	//Bind to the the dpdk driver
	err = ioutil.WriteFile(bindDevicePath, []byte(vendorPrefix+vfdevID), 0600)
	if err != nil {
		return errors.Wrapf(err, "Binding to the DPDK driver failed for the device %s", vfBdf)
	}
	return nil
}

func isValidKerneDriver(kernelvfDriver string) bool {
	switch kernelvfDriver {
	case "dh895xccvf", "c6xxvf", "c3xxxvf", "d15xxvf", "c4xxvf", "4xxxvf":
		return true
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
func isValidVfDeviceID(vfDevID string) bool {
	switch vfDevID {
	case "0442", "0443", "37c9", "19e3", "4941", "18a1":
		return true
	}
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

func (dp *DevicePlugin) scan() (dpapi.DeviceTree, error) {
	devTree := dpapi.NewDeviceTree()
	n := 0
	for _, driver := range append([]string{dp.dpdkDriver}, dp.kernelVfDrivers...) {
		files, err := ioutil.ReadDir(filepath.Join(dp.pciDriverDir, driver))
		if err != nil {
			klog.Warningf("Can't read sysfs for driver as Driver %s is not available: Skipping", driver)
			continue
		}

		for _, file := range files {
			if !strings.HasPrefix(file.Name(), "0000:") {
				continue
			}
			vfdevID, err := dp.getDeviceID(file.Name())
			if err != nil {
				return nil, errors.Wrapf(err, "Cannot obtain deviceID for the device with PCI address: %s", file.Name())
			}
			if !isValidVfDeviceID(vfdevID) {
				continue
			}
			n = n + 1 // increment after all junk got filtered out

			if n > dp.maxDevices {
				break
			}

			// initialize newly found devices which aren't bound to DPDK driver yet
			if driver != dp.dpdkDriver {
				err = dp.bindDevice(file.Name())
				if err != nil {
					return nil, err
				}
			}

			dpdkDeviceName, err := dp.getDpdkDevice(file.Name())
			if err != nil {
				return nil, err
			}

			klog.V(1).Infof("%s device: corresponding DPDK device detected is %s", file.Name(), dpdkDeviceName)

			envs := map[string]string{
				fmt.Sprintf("%s%d", envVarPrefix, n): file.Name(),
			}

			devinfo := dpapi.NewDeviceInfo(pluginapi.Healthy, dp.getDpdkDeviceSpecs(dpdkDeviceName), dp.getDpdkMounts(dpdkDeviceName), envs)

			devTree.AddDevice("generic", file.Name(), devinfo)
		}
	}

	return devTree, nil
}
