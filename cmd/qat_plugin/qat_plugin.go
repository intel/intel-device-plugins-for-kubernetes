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
	"path/filepath"
	"strconv"
	"strings"
	"path"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

const (
	uioDevicePath         = "/dev/"
	vfioDevicePath        = "/dev/vfio/"
	uioMountPath          = "/sys/class/uio/"
	pciDeviceDir          = "/sys/bus/pci/devices/"
	pciDriverDir          = "/sys/bus/pci/drivers/"
	uioSuffix             = "/uio"
	iommuGroupSuffix      = "/iommu_group"
	sysfsIommuGroupSuffix = "/sys/kernel/iommu_groups/"
	newIDSuffix           = "/new_id"
	driverUnbindSuffix    = "/driver/unbind"
	qatDeviceRE           = "[0-9|a-f][0-9|a-f]:[0-9|a-f][0-9|a-f].[0-9|a-f].*"
	vendorPrefix          = "8086 "

	// Device plugin settings.
	pluginEndpointPrefix = "intelQAT"
	resourceName         = "intel.com/qat"
)

var (
	dpdkDriver      = flag.String("dpdk-driver", "igb_uio", "DPDK Device driver for configuring the QAT device")
	kernelVfDrivers = flag.String("kernel-vf-drivers", "dh895xccvf,c6xxvf,c3xxxvf,d15xxvf", "Comma separated VF Device Driver of the QuickAssist Devices in the system. Devices supported: DH895xCC,C62x,C3xxx and D15xx")
	maxNumdevice    = flag.String("max-num-devices", "32", "maximum number of QAT devices to be provided to the QuickAssist device plugin")
)

// deviceManager manages Intel gpu devices.
type deviceManager struct {
	srv     deviceplugin.Server
	devices map[string]deviceplugin.DeviceInfo
}

func newDeviceManager() (*deviceManager, error) {
	return &deviceManager{
		devices: make(map[string]deviceplugin.DeviceInfo),
	}, nil
}

func getDpdkDevice(id string) (string, error) {

	devicePCIAdd := "0000:" + id
	switch *dpdkDriver {

	case "igb_uio":
		uioDirPath := path.Join(pciDeviceDir, devicePCIAdd, uioSuffix)
		files, err := ioutil.ReadDir(uioDirPath)
		if err != nil {
			return "", fmt.Errorf("Error in reading the uioDirPath")
		}
		return files[0].Name(), nil

	case "vfio-pci":
		vfioDirPath := path.Join(pciDeviceDir, devicePCIAdd, iommuGroupSuffix)
		group, err := filepath.EvalSymlinks(vfioDirPath)
		if err != nil {
			return "", fmt.Errorf("Error in evaluating symlink for the vfiopath")
		}
		s := strings.TrimPrefix(group, sysfsIommuGroupSuffix)
		fmt.Printf("The vfio device group detected is %v\n", s)
		return s, nil
	// TODO: case "pci-generic" and "kernel":
	default: //fmt.Printf("Please enter the dpdk Driver correctly")
		return "", fmt.Errorf("Please enter the dpdk Driver correctly")
	}

}
func getDpdkDeviceNames(id string) ([]string, error) {
	dpdkDeviceName, err := getDpdkDevice(id)
	fmt.Printf("%v device: corresponding DPDK device detected is %v\n", id, dpdkDeviceName)
	if err != nil {
		return []string{}, fmt.Errorf("Unable to get the dpdk device for creating device nodes: %v", err)
	}
	switch *dpdkDriver {
	case "", "igb_uio":
		//Setting up with uio
		uioDev := path.Join(uioDevicePath, dpdkDeviceName)
		return []string{uioDev}, nil
	case "vfio-pci":
		//Setting up with vfio
		vfioDev1 := path.Join(vfioDevicePath, dpdkDeviceName)
		vfioDev2 := path.Join(vfioDevicePath, "/vfio")
		return []string{vfioDev1, vfioDev2}, nil
	// TODO: case "pci-generic" and "kernel":
	default:
		return []string{}, fmt.Errorf("Please enter the dpdk Driver correctly")
	}
}
func getDpdkMountPaths(id string) ([]string, error) {
	dpdkDeviceName, err := getDpdkDevice(id)
	if err != nil {
		return []string{}, fmt.Errorf("Unable to get the dpdk device for mountPath: %v", err)
	}
	switch *dpdkDriver {
	case "igb_uio":
		//Setting up with uio mountpoints
		uioMountPoint := path.Join(uioMountPath, dpdkDeviceName, "/device")
		return []string{uioMountPoint}, nil
	case "vfio-pci":
		//No mountpoint for vfio needs to be populated
		return []string{}, nil
	default:
		return nil, fmt.Errorf("Please enter the dpdk Driver correctly")
	}
}

//identify the device iD of a device
func getDeviceID(pciAddr string) (string, error) {
	deviceIDPath := path.Join(pciDeviceDir, pciAddr, "/device")
	devID, err := ioutil.ReadFile(deviceIDPath)
	if err != nil {
		return "", fmt.Errorf("Cannot obtain the Device ID for this device: %v", err)
	}
	id := bytes.TrimSpace(devID)
	idStr := strings.TrimPrefix(string(id), "0x")
	return idStr, nil
}

// bindDevice the device where id is the pci address to the specified device driver
func bindDevice(dpdkDriver string, id string) error {

	devicePCIAddr := "0000:"+id
	unbindKernelDevicePath := path.Join(pciDeviceDir, devicePCIAddr, driverUnbindSuffix)
	bindDevicePath := path.Join(pciDriverDir, dpdkDriver, newIDSuffix)
	devicePCIAddrBytes := []byte(devicePCIAddr)
	vfdevID, err := getDeviceID(devicePCIAddr)
	if err != nil {
		glog.Error(err)
		fmt.Printf("Cannot obtain the Device ID for this device")
		return fmt.Errorf("Cannot obtain the Device ID for this device: %v", err)
	}
	err = ioutil.WriteFile(unbindKernelDevicePath, devicePCIAddrBytes, 0644)
	if err != nil {
		glog.Error(err)
		fmt.Printf("Unbinding from the kernel driver failed\n")
		return fmt.Errorf("Unbinding from the kernel driver failed: %v", err)

	}
	// Unbinding from the kernel driver DONE
	err = ioutil.WriteFile(bindDevicePath, []byte(vendorPrefix+vfdevID), 0644)
	if err != nil {
		fmt.Printf("Binding to the dpdk driver failed\n")
		return fmt.Errorf("Binding to the dpdk driver failed: %v", err)
	}
	//Binding to the the dpdk driver DONE\n
	return nil
}
func isValidKerneDriver(kernelvfDriver string) error {
	switch kernelvfDriver {
	case "dh895xccvf", "c6xxvf", "c3xxxvf", "d15xxvf":
		return nil
	default:
		return fmt.Errorf("Please enter the kernel-vf-drivers flag correctly")
	}
}
func isValidDpdkDeviceDriver(dpdkDriver string) error {
	switch dpdkDriver {
	case "igb_uio", "vfio-pci":
		return nil
	default:
		return fmt.Errorf("Please enter the dpdk-driver flag correctly")
	}
}

// Discovers all QAT devices available on the local node by querying PCI bus using lspci.
func (dm *deviceManager) discoverQATs() (bool, error) {

	found := false
	fmt.Println("Discovered Devices below:")
	kernelvfDrivers := strings.Split(*kernelVfDrivers, ",")

	for _, kernelvfDriver := range kernelvfDrivers {
		err := isValidKerneDriver(kernelvfDriver)
		if err != nil {
			return found, fmt.Errorf("Error in user input for kernel VF Driver: %v", err)
		}

		files, err := ioutil.ReadDir(path.Join(pciDriverDir, kernelvfDriver))
		if err != nil {
			fmt.Printf("Can't read sysfs for kernel vf driver %v: %v", kernelvfDriver, err)
			continue
		}

		for n, file := range files {
			if strings.HasPrefix(file.Name(), "0000:") {
				vfpciaddr := strings.TrimPrefix(file.Name(), "0000:")
				max, err := strconv.Atoi(*maxNumdevice)
				if err != nil {
					return found, fmt.Errorf("Error in getting maximum number of devices: %v", err)
				}

				if n < max {
					err = bindDevice(*dpdkDriver, vfpciaddr)
					if err != nil {
						return found, fmt.Errorf("Error in binding the device to the dpdk driver")
					}
					devNodes, err := getDpdkDeviceNames(vfpciaddr)
					if err != nil {
						return found, fmt.Errorf("Error in obtaining the device name ")
					}
					devMountPoints, err := getDpdkMountPaths(vfpciaddr)
					if err != nil {
						return found, fmt.Errorf("Error in obtaining the mount point ")
					}

					dev := deviceplugin.DeviceInfo{pluginapi.Healthy, devNodes, devMountPoints}
					dm.devices[vfpciaddr] = dev

				}
			}

		}

	}

	fmt.Printf("The number of devices configured are:%v\n", len(dm.devices))

	if len(dm.devices) > 0 {
		found = true
	}

	return found, nil
}

func (dm *deviceManager) getDeviceState(DeviceName string) string {
	// TODO: calling tools to figure out actual device state
	return pluginapi.Healthy
}

// Implements DevicePlugin service functions
func (dm *deviceManager) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	fmt.Println("GetDevicePluginOptions: return empty options")
	return new(pluginapi.DevicePluginOptions), nil
}

func (dm *deviceManager) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	changed := true
	for {
		for id, dev := range dm.devices {
			state := dm.getDeviceState(id)
			if dev.State != state {
				changed = true
				dev.State = state
				dm.devices[id] = dev
			}
		}
		if changed {
			resp := new(pluginapi.ListAndWatchResponse)
			for id, dev := range dm.devices {
				resp.Devices = append(resp.Devices, &pluginapi.Device{id, dev.State})
			}
			fmt.Printf("ListAndWatch: Sending device response\n")
			if err := stream.Send(resp); err != nil {
				dm.srv.Stop()
				return fmt.Errorf("device-plugin: cannot update device states: %v", err)
			}
		}
		changed = false
		time.Sleep(5 * time.Second)
	}
}

func (dm *deviceManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	resp, err := deviceplugin.MakeAllocateResponse(rqt, dm.devices)
	if err != nil {
		return nil, err
	}

	for cnum, crqt := range rqt.ContainerRequests {
		envmap := make(map[string]string)

		for devNum, id := range crqt.DevicesIDs {
			envmap[fmt.Sprintf("%s%d", pluginEndpointPrefix, devNum+1)] = "0000:" + id

			for _, mountPoint := range dm.devices[id].DeviceMountPath {
				fmt.Printf("mountDir mounting is %v\n", mountPoint)
				resp.ContainerResponses[cnum].Mounts = append(resp.ContainerResponses[cnum].Mounts, &pluginapi.Mount{
					HostPath:      mountPoint,
					ContainerPath: mountPoint,
					ReadOnly:      false,
				})
			}
		}

		resp.ContainerResponses[cnum].Envs = envmap
	}

	return resp, nil
}

func (dm *deviceManager) PreStartContainer(ctx context.Context, rqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	glog.Warning("PreStartContainer() should not be called")
	return new(pluginapi.PreStartContainerResponse), nil
}

func main() {
	flag.Parse()
	fmt.Println("QAT device plugin started")
	
	err := isValidDpdkDeviceDriver(*dpdkDriver)
	if err != nil {
		glog.Fatalf("Error in user input for DPDK Device Driver: %v", err)
	}
	_, err = strconv.Atoi(*maxNumdevice)
	if err != nil {
		glog.Fatalf("Error in getting maximum number of devices: %v", err)
	}
	dm, err := newDeviceManager()
	if err != nil {
		glog.Fatal(err)
		os.Exit(1)
	}
	found, err := dm.discoverQATs()
	if err != nil {
		glog.Fatalf("Error in discovery process %v\n", err)
	}
	if !found {
		glog.Fatalf("No QAT devices configured\n")
	}

	err = dm.srv.Serve(dm, resourceName, pluginEndpointPrefix)
	if err != nil {
		glog.Fatal(err)
	}
}
