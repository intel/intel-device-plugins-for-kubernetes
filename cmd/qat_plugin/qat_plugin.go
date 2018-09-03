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

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
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

	namespace = "qat.intel.com"
)

type devicePlugin struct {
	maxDevices      int
	pciDriverDir    string
	pciDeviceDir    string
	kernelVfDrivers []string
	dpdkDriver      string
}

func newDevicePlugin(pciDriverDir, pciDeviceDir string, maxDevices int, kernelVfDrivers []string, dpdkDriver string) *devicePlugin {
	return &devicePlugin{
		maxDevices:      maxDevices,
		pciDriverDir:    pciDriverDir,
		pciDeviceDir:    pciDeviceDir,
		kernelVfDrivers: kernelVfDrivers,
		dpdkDriver:      dpdkDriver,
	}
}

func (dp *devicePlugin) Scan(notifier deviceplugin.Notifier) error {
	for {
		devTree, err := dp.scan()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		time.Sleep(5 * time.Second)
	}
}

func (dp *devicePlugin) getDpdkDevice(id string) (string, error) {

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
		fmt.Printf("The vfio device group detected is %v\n", s)
		return s, nil
	}

	return "", errors.New("Unknown DPDK driver")
}

func (dp *devicePlugin) getDpdkDeviceNames(id string) ([]string, error) {
	dpdkDeviceName, err := dp.getDpdkDevice(id)
	if err != nil {
		return []string{}, err
	}
	fmt.Printf("%s device: corresponding DPDK device detected is %s\n", id, dpdkDeviceName)

	switch dp.dpdkDriver {
	// TODO: case "pci-generic" and "kernel":
	case "igb_uio":
		//Setting up with uio
		uioDev := path.Join(uioDevicePath, dpdkDeviceName)
		return []string{uioDev}, nil
	case "vfio-pci":
		//Setting up with vfio
		vfioDev1 := path.Join(vfioDevicePath, dpdkDeviceName)
		vfioDev2 := path.Join(vfioDevicePath, "/vfio")
		return []string{vfioDev1, vfioDev2}, nil
	}

	return []string{}, errors.New("Unknown DPDK driver")
}

func (dp *devicePlugin) getDpdkMountPaths(id string) ([]string, error) {
	dpdkDeviceName, err := dp.getDpdkDevice(id)
	if err != nil {
		return []string{}, err
	}

	switch dp.dpdkDriver {
	case "igb_uio":
		//Setting up with uio mountpoints
		uioMountPoint := path.Join(uioMountPath, dpdkDeviceName, "/device")
		return []string{uioMountPoint}, nil
	case "vfio-pci":
		//No mountpoint for vfio needs to be populated
		return []string{}, nil
	}

	return nil, errors.New("Unknown DPDK driver")
}

func (dp *devicePlugin) getDeviceID(pciAddr string) (string, error) {
	devID, err := ioutil.ReadFile(path.Join(dp.pciDeviceDir, pciAddr, "device"))
	if err != nil {
		return "", errors.Wrapf(err, "Cannot obtain ID for the device %s", pciAddr)
	}

	return strings.TrimPrefix(string(bytes.TrimSpace(devID)), "0x"), nil
}

// bindDevice unbinds given device from kernel driver and binds to DPDK driver
func (dp *devicePlugin) bindDevice(id string) error {
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

func (dp *devicePlugin) scan() (deviceplugin.DeviceTree, error) {
	devTree := deviceplugin.NewDeviceTree()

	for _, driver := range append(dp.kernelVfDrivers, dp.dpdkDriver) {
		files, err := ioutil.ReadDir(path.Join(dp.pciDriverDir, driver))
		if err != nil {
			fmt.Printf("Can't read sysfs for driver as Driver %s is not available: Skipping\n", driver)
			continue
		}

		n := 0
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

			devNodes, err := dp.getDpdkDeviceNames(vfpciaddr)
			if err != nil {
				return nil, err
			}
			devMounts, err := dp.getDpdkMountPaths(vfpciaddr)
			if err != nil {
				return nil, err
			}
			deviceName := strings.TrimSuffix(namespace, ".intel.com")

			devinfo := deviceplugin.DeviceInfo{
				State:  pluginapi.Healthy,
				Nodes:  devNodes,
				Mounts: devMounts,
				Envs: map[string]string{
					fmt.Sprintf("%s%d", strings.ToUpper(deviceName), n): file.Name(),
				},
			}

			devTree.AddDevice("generic", vfpciaddr, devinfo)
		}
	}

	return devTree, nil
}

func main() {
	dpdkDriver := flag.String("dpdk-driver", "igb_uio", "DPDK Device driver for configuring the QAT device")
	kernelVfDrivers := flag.String("kernel-vf-drivers", "dh895xccvf,c6xxvf,c3xxxvf,d15xxvf", "Comma separated VF Device Driver of the QuickAssist Devices in the system. Devices supported: DH895xCC,C62x,C3xxx and D15xx")
	maxNumDevices := flag.Int("max-num-devices", 32, "maximum number of QAT devices to be provided to the QuickAssist device plugin")
	debugEnabled := flag.Bool("debug", false, "enable debug output")
	flag.Parse()
	fmt.Println("QAT device plugin started")

	if *debugEnabled {
		debug.Activate()
	}

	if !isValidDpdkDeviceDriver(*dpdkDriver) {
		fmt.Println("Wrong DPDK device driver:", *dpdkDriver)
		os.Exit(1)
	}

	kernelDrivers := strings.Split(*kernelVfDrivers, ",")
	for _, driver := range kernelDrivers {
		if !isValidKerneDriver(driver) {
			fmt.Println("Wrong kernel VF driver:", driver)
			os.Exit(1)
		}
	}

	plugin := newDevicePlugin(pciDriverDirectory, pciDeviceDirectory, *maxNumDevices, kernelDrivers, *dpdkDriver)
	manager := deviceplugin.NewManager(namespace, plugin)
	manager.Run()
}
