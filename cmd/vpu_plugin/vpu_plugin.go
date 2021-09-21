// Copyright 2020 Intel Corporation. All Rights Reserved.
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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/gousb"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// Movidius MyriadX Vendor ID.
	vendorID = 0x03e7
	// Device plugin settings.
	namespace  = "vpu.intel.com"
	deviceType = "hddl"

	hddlSockPath     = "/var/tmp/hddl_service.sock"
	hddlServicePath1 = "/var/tmp/hddl_service_ready.mutex"
	hddlServicePath2 = "/var/tmp/hddl_service_alive.mutex"
	ionDevNode       = "/dev/ion"
	// Frequency of device scans.
	scanFrequency = 5 * time.Second
)

const (
	vendorIDIntel = "0x8086"
	xlinkDevNode  = "/dev/xlnk"

	hddlAlive     = "/var/tmp/hddlunite_service_alive.mutex"
	hddlReady     = "/var/tmp/hddlunite_service_ready.mutex"
	hddlStartExit = "/var/tmp/hddlunite_service_start_exit.mutex"
	hddlSocketPci = "/var/tmp/hddlunite_service.sock"

	sysBusPCIDevice = "/sys/bus/pci/devices"
)

var (
	// Movidius MyriadX Product IDs.
	productIDs = []int{0x2485, 0xf63b}
	// PCI Product IDs.
	productIDsPCI = []PCIPidDeviceType{{"kmb", []string{"0x6240"}, 1}}
)

type gousbContext interface {
	OpenDevices(opener func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error)
}

type PCIPidDeviceType struct {
	deviceType string
	pids       []string
	ratio      int
}

func getPciDeviceCounts(sysfsPciDevicesPath string, vendorID string, pidSearch []PCIPidDeviceType) ([]int, error) {
	found := make([]int, len(pidSearch))

	bdf, _ := os.ReadDir(sysfsPciDevicesPath)
	// Check for all folder inside sysfs
	for _, bus := range bdf {
		// Extract vid and pid
		vidRaw, _ := os.ReadFile(filepath.Join(sysfsPciDevicesPath, bus.Name(), "vendor"))
		pidRaw, _ := os.ReadFile(filepath.Join(sysfsPciDevicesPath, bus.Name(), "device"))
		vid := strings.TrimSpace(string(vidRaw))
		pid := strings.TrimSpace(string(pidRaw))
		// Loop for supported VPU type: kmb
		for i, pciPid := range pidSearch {
			// Loop for list of pid of supported device type
			for _, pidVPU := range pciPid.pids {
				if vid == vendorID && pid == pidVPU {
					found[i]++
				}
			}
		}
	}
	return found, nil
}

type devicePlugin struct {
	deviceCtx    interface{}
	scanTicker   *time.Ticker
	scanDone     chan bool
	sharedDevNum int
}

type devicePluginUsb struct {
	usbContext gousbContext
	productIDs []int
	vendorID   int
}

type devicePluginPci struct {
	sysfsPciDevicesPath string
	vendorIDPCI         string
	productIDsPCI       []PCIPidDeviceType
}

func newDevicePlugin(deviceCtx interface{}, sharedDevNum int) *devicePlugin {
	if sharedDevNum < 1 {
		klog.V(1).Info("The number of containers sharing the same VPU must greater than zero")
		return nil
	}
	return &devicePlugin{
		deviceCtx:    deviceCtx,
		sharedDevNum: sharedDevNum,
		scanTicker:   time.NewTicker(scanFrequency),
		scanDone:     make(chan bool, 1),
	}
}

func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()
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

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err == nil && info != nil {
		return !info.IsDir()
	}
	// regard all other case as abnormal
	return false
}

func (dp *devicePlugin) scanUsb(devTree *dpapi.DeviceTree) {
	var nUsb int

	// first check if HDDL sock is there
	if !fileExists(hddlSockPath) {
		return
	}

	deviceCtx, ok := dp.deviceCtx.(devicePluginUsb)
	if !ok {
		klog.V(4).Infof("wrong context %s", ok)
	}

	devs, err := deviceCtx.usbContext.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		thisVendor := desc.Vendor
		thisProduct := desc.Product
		for _, v := range deviceCtx.productIDs {
			klog.V(4).Infof("checking %04x,%04x vs %s,%s", deviceCtx.vendorID, v, thisVendor.String(), thisProduct.String())
			if (gousb.ID(deviceCtx.vendorID) == thisVendor) && (gousb.ID(v) == thisProduct) {
				nUsb++
			}
		}
		return false
	})
	defer func() {
		for _, d := range devs {
			d.Close()
		}
	}()

	if err != nil {
		klog.V(4).Infof("list usb device %s", err)
	}

	if nUsb > 0 {
		for i := 0; i < nUsb*dp.sharedDevNum; i++ {
			devID := fmt.Sprintf("hddl_service-%d", i)
			// HDDL use a unix socket as service provider to manage /dev/myriad[n]
			// Here we only expose an ION device to be allocated for HDDL client in containers
			nodes := []pluginapi.DeviceSpec{
				{
					HostPath:      ionDevNode,
					ContainerPath: ionDevNode,
					Permissions:   "rw",
				},
			}

			mounts := []pluginapi.Mount{
				{
					HostPath:      hddlSockPath,
					ContainerPath: hddlSockPath,
				},
				{
					HostPath:      hddlServicePath1,
					ContainerPath: hddlServicePath1,
				},
				{
					HostPath:      hddlServicePath2,
					ContainerPath: hddlServicePath2,
				},
			}
			devTree.AddDevice(deviceType, devID, dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, mounts, nil))
		}
	}
}

func (dp *devicePlugin) scanPci(devTree *dpapi.DeviceTree) {
	// first check if HDDL sock is there
	if !fileExists(hddlSocketPci) {
		return
	}

	deviceCtx, ok := dp.deviceCtx.(devicePluginPci)
	if !ok {
		klog.V(4).Infof("wrong context %s", ok)
	}
	// Get all PCI devices
	pciFound, err := getPciDeviceCounts(deviceCtx.sysfsPciDevicesPath, deviceCtx.vendorIDPCI, deviceCtx.productIDsPCI)

	if err != nil {
		klog.V(4).Infof("list pci device %s", err)
	}

	// Mount VPU
	for i := 0; i < len(pciFound); i++ {
		deviceTypePci := deviceCtx.productIDsPCI[i].deviceType
		deviceRatio := deviceCtx.productIDsPCI[i].ratio
		// If device found
		if remainder := pciFound[i] % deviceRatio; remainder == 0 {
			count := pciFound[i] / deviceRatio
			nodes := []pluginapi.DeviceSpec{
				{
					HostPath:      xlinkDevNode,
					ContainerPath: xlinkDevNode,
					Permissions:   "rw",
				},
			}

			mounts := []pluginapi.Mount{
				{
					HostPath:      hddlAlive,
					ContainerPath: hddlAlive,
				},
				{
					HostPath:      hddlReady,
					ContainerPath: hddlReady,
				},
				{
					HostPath:      hddlStartExit,
					ContainerPath: hddlStartExit,
				},
				{
					HostPath:      hddlSocketPci,
					ContainerPath: hddlSocketPci,
				},
			}
			// Mount all devices
			for i := 0; i < count; i++ {
				devID := fmt.Sprintf("%s-device-%d", deviceTypePci, i)
				// VPU pci device found and added to node
				klog.V(1).Info(devID)
				devTree.AddDevice(deviceTypePci, devID, dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, mounts, nil))
			}
		}
	}
}

func (dp *devicePlugin) scan() (dpapi.DeviceTree, error) {
	devTree := dpapi.NewDeviceTree()

	switch dp.deviceCtx.(type) {
	case devicePluginUsb:
		dp.scanUsb(&devTree)
	case devicePluginPci:
		dp.scanPci(&devTree)
	default:
	}

	return devTree, nil
}

func main() {
	var sharedDevNum int
	var scanMode int

	flag.IntVar(&sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same VPU device")
	flag.IntVar(&scanMode, "mode", 1, "USB=1 PCI=2")
	flag.Parse()

	klog.V(1).Info("VPU device plugin started")

	var plugin *devicePlugin
	if scanMode == 1 {
		// add lsusb here
		ctx := gousb.NewContext()
		defer ctx.Close()

		verbosityLevel, err := strconv.Atoi(flag.CommandLine.Lookup("v").Value.String())
		if err == nil {
			// gousb (libusb) Debug levels are a 1:1 match to klog levels, just pass through.
			ctx.Debug(verbosityLevel)
		}
		deviceCtxUsb := devicePluginUsb{usbContext: ctx, vendorID: vendorID, productIDs: productIDs}
		plugin = newDevicePlugin(deviceCtxUsb, sharedDevNum)
	} else if scanMode == 2 {
		deviceCtxPci := devicePluginPci{sysfsPciDevicesPath: sysBusPCIDevice, vendorIDPCI: vendorIDIntel, productIDsPCI: productIDsPCI}
		plugin = newDevicePlugin(deviceCtxPci, sharedDevNum)
	}
	if plugin == nil {
		klog.Fatal("Cannot create device plugin, please check above error messages.")
	}
	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
