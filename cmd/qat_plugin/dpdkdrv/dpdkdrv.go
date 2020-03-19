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

package dpdkdrv

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"
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
	uioMountPath       = "/sys/class/uio"
	pciDeviceDirectory = "/sys/bus/pci/devices"
	pciDriverDirectory = "/sys/bus/pci/drivers"
	uioSuffix          = "uio"
	iommuGroupSuffix   = "iommu_group"
	newIDSuffix        = "new_id"
	driverUnbindSuffix = "driver/unbind"
	vendorPrefix       = "8086 "
	envVarPrefix       = "QAT"
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

func (dp *DevicePlugin) getDpdkDevice(id string) (string, error) {

	devicePCIAdd := "0000:" + id
	switch dp.dpdkDriver {
	// TODO: case "pci-generic" and "kernel":
	case "igb_uio":
		uioDirPath := path.Join(dp.pciDeviceDir, devicePCIAdd, uioSuffix)
		files, err := ioutil.ReadDir(uioDirPath)
		if err != nil {
			return "", err
		}
		if len(files) == 0 {
			return "", errors.New("No devices found")
		}
		return files[0].Name(), nil

	case "vfio-pci":
		vfioDirPath := path.Join(dp.pciDeviceDir, devicePCIAdd, iommuGroupSuffix)
		group, err := filepath.EvalSymlinks(vfioDirPath)
		if err != nil {
			return "", errors.WithStack(err)
		}
		s := path.Base(group)
		klog.V(1).Infof("The vfio device group detected is %v", s)
		return s, nil
	}

	return "", errors.New("Unknown DPDK driver")
}

func (dp *DevicePlugin) getDpdkDeviceSpecs(id string) ([]pluginapi.DeviceSpec, error) {
	dpdkDeviceName, err := dp.getDpdkDevice(id)
	if err != nil {
		return nil, err
	}
	klog.V(1).Infof("%s device: corresponding DPDK device detected is %s", id, dpdkDeviceName)

	switch dp.dpdkDriver {
	// TODO: case "pci-generic" and "kernel":
	case "igb_uio":
		//Setting up with uio
		uioDev := path.Join(uioDevicePath, dpdkDeviceName)
		return []pluginapi.DeviceSpec{
			{
				HostPath:      uioDev,
				ContainerPath: uioDev,
				Permissions:   "rw",
			},
		}, nil
	case "vfio-pci":
		//Setting up with vfio
		vfioDev1 := path.Join(vfioDevicePath, dpdkDeviceName)
		vfioDev2 := path.Join(vfioDevicePath, "/vfio")
		return []pluginapi.DeviceSpec{
			{
				HostPath:      vfioDev1,
				ContainerPath: vfioDev1,
				Permissions:   "rw",
			},
			{
				HostPath:      vfioDev2,
				ContainerPath: vfioDev2,
				Permissions:   "rw",
			},
		}, nil
	}

	return nil, errors.New("Unknown DPDK driver")
}

func (dp *DevicePlugin) getDpdkMounts(id string) ([]pluginapi.Mount, error) {
	dpdkDeviceName, err := dp.getDpdkDevice(id)
	if err != nil {
		return nil, err
	}

	switch dp.dpdkDriver {
	case "igb_uio":
		//Setting up with uio mountpoints
		uioMountPoint := path.Join(uioMountPath, dpdkDeviceName, "/device")
		return []pluginapi.Mount{
			{
				HostPath:      uioMountPoint,
				ContainerPath: uioMountPath,
			},
		}, nil
	case "vfio-pci":
		//No mountpoint for vfio needs to be populated
		return nil, nil
	}

	return nil, errors.New("Unknown DPDK driver")
}

func (dp *DevicePlugin) getDeviceID(pciAddr string) (string, error) {
	devID, err := ioutil.ReadFile(path.Join(dp.pciDeviceDir, pciAddr, "device"))
	if err != nil {
		return "", errors.Wrapf(err, "Cannot obtain ID for the device %s", pciAddr)
	}

	return strings.TrimPrefix(string(bytes.TrimSpace(devID)), "0x"), nil
}

// bindDevice unbinds given device from kernel driver and binds to DPDK driver
func (dp *DevicePlugin) bindDevice(id string) error {
	devicePCIAddr := "0000:" + id
	unbindDevicePath := path.Join(dp.pciDeviceDir, devicePCIAddr, driverUnbindSuffix)

	// Unbind from the kernel driver
	err := ioutil.WriteFile(unbindDevicePath, []byte(devicePCIAddr), 0644)
	if err != nil {
		return errors.Wrapf(err, "Unbinding from kernel driver failed for the device %s", id)

	}
	vfdevID, err := dp.getDeviceID(devicePCIAddr)
	if err != nil {
		return err
	}
	bindDevicePath := path.Join(dp.pciDriverDir, dp.dpdkDriver, newIDSuffix)
	//Bind to the the dpdk driver
	err = ioutil.WriteFile(bindDevicePath, []byte(vendorPrefix+vfdevID), 0644)
	if err != nil {
		return errors.Wrapf(err, "Binding to the DPDK driver failed for the device %s", id)
	}
	return nil
}

func isValidKerneDriver(kernelvfDriver string) bool {
	switch kernelvfDriver {
	case "dh895xccvf", "c6xxvf", "c3xxxvf", "d15xxvf":
		return true
	}
	return false
}

func isValidDpdkDeviceDriver(dpdkDriver string) bool {
	switch dpdkDriver {
	case "igb_uio", "vfio-pci":
		return true
	}
	return false
}
func isValidVfDeviceID(vfDevID string) bool {
	switch vfDevID {
	case "0442", "0443", "37c9", "19e3":
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
			tempMap[strings.Join([]string{"QAT", strconv.Itoa(counter)}, "")] = cresp.Envs[k]
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
		files, err := ioutil.ReadDir(path.Join(dp.pciDriverDir, driver))
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

			vfpciaddr := strings.TrimPrefix(file.Name(), "0000:")

			// initialize newly found devices which aren't bound to DPDK driver yet
			if driver != dp.dpdkDriver {
				err = dp.bindDevice(vfpciaddr)
				if err != nil {
					return nil, err
				}
			}

			devNodes, err := dp.getDpdkDeviceSpecs(vfpciaddr)
			if err != nil {
				return nil, err
			}
			devMounts, err := dp.getDpdkMounts(vfpciaddr)
			if err != nil {
				return nil, err
			}

			envs := map[string]string{
				fmt.Sprintf("%s%d", envVarPrefix, n): file.Name(),
			}
			devinfo := dpapi.NewDeviceInfo(pluginapi.Healthy, devNodes, devMounts, envs)

			devTree.AddDevice("generic", vfpciaddr, devinfo)
		}
	}

	return devTree, nil
}
