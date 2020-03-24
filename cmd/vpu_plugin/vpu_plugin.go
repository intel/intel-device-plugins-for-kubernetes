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
	"strconv"
	"time"

	"github.com/google/gousb"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// Movidius MyriadX Vendor ID
	vendorID = 0x03e7
	// Device plugin settings.
	namespace  = "vpu.intel.com"
	deviceType = "hddl"

	hddlSockPath     = "/var/tmp/hddl_service.sock"
	hddlServicePath1 = "/var/tmp/hddl_service_ready.mutex"
	hddlServicePath2 = "/var/tmp/hddl_service_alive.mutex"
	ionDevNode       = "/dev/ion"
)

var (
	// Movidius MyriadX Product IDs
	productIDs = []int{0x2485, 0xf63b}
)

type gousbContext interface {
	OpenDevices(opener func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error)
}

type devicePlugin struct {
	usbContext   gousbContext
	vendorID     int
	productIDs   []int
	sharedDevNum int
}

func newDevicePlugin(usbContext gousbContext, vendorID int, productIDs []int, sharedDevNum int) *devicePlugin {
	return &devicePlugin{
		usbContext:   usbContext,
		vendorID:     vendorID,
		productIDs:   productIDs,
		sharedDevNum: sharedDevNum,
	}
}

func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	for {
		devTree, err := dp.scan()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		time.Sleep(5 * time.Second)
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

func (dp *devicePlugin) scan() (dpapi.DeviceTree, error) {
	var nUsb int
	devTree := dpapi.NewDeviceTree()

	// first check if HDDL sock is there
	if !fileExists(hddlSockPath) {
		return devTree, nil
	}

	devs, err := dp.usbContext.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		thisVendor := desc.Vendor
		thisProduct := desc.Product
		for _, v := range dp.productIDs {
			klog.V(4).Infof("checking %04x,%04x vs %s,%s", dp.vendorID, v, thisVendor.String(), thisProduct.String())
			if (gousb.ID(dp.vendorID) == thisVendor) && (gousb.ID(v) == thisProduct) {
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

	return devTree, nil
}

func main() {
	var sharedDevNum int

	flag.IntVar(&sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same VPU device")

	flag.Parse()

	klog.V(4).Info("debug is on")

	if sharedDevNum < 1 {
		klog.Fatal("The number of containers sharing the same VPU must greater than zero")
		os.Exit(1)
	}

	klog.V(1).Info("VPU device plugin started")

	// add lsusb here
	ctx := gousb.NewContext()
	defer ctx.Close()

	verbosityLevel, err := strconv.Atoi(flag.CommandLine.Lookup("v").Value.String())
	if err == nil {
		// gousb (libusb) Debug levels are a 1:1 match to klog levels, just pass through.
		ctx.Debug(verbosityLevel)
	}

	plugin := newDevicePlugin(ctx, vendorID, productIDs, sharedDevNum)
	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
