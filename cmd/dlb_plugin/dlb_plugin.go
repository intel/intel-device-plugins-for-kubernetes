// Copyright 2021 Intel Corporation. All Rights Reserved.
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
	"path/filepath"
	"reflect"
	"time"

	"k8s.io/klog/v2"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/pluginutils"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	dlbDeviceFilePathRE = "/dev/dlb*"
	namespace           = "dlb.intel.com"
	deviceTypePF        = "pf"
	deviceTypeVF        = "vf"
	sysfsDir            = "/sys/class/dlb2"
	// Period of device scans.
	scanPeriod = 5 * time.Second
)

type DevicePlugin struct {
	scanTicker *time.Ticker
	scanDone   chan bool

	dlbDeviceFilePathReg string
	sysfsDir             string
}

func NewDevicePlugin(dlbDeviceFilePathReg string, sysfsDir string) *DevicePlugin {
	return &DevicePlugin{
		dlbDeviceFilePathReg: dlbDeviceFilePathReg,
		sysfsDir:             sysfsDir,
		scanTicker:           time.NewTicker(scanPeriod),
		scanDone:             make(chan bool, 1), // buffered as we may send to it before Scan starts receiving from it
	}
}

func (dp *DevicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	var prevDevTree dpapi.DeviceTree

	for {
		devTree := dp.scan()

		if !reflect.DeepEqual(prevDevTree, devTree) {
			klog.V(1).Info("DLB scan update: pf: ", len(devTree[deviceTypePF]), " / vf: ", len(devTree[deviceTypeVF]))
			prevDevTree = devTree
		}

		notifier.Notify(devTree)

		select {
		case <-dp.scanDone:
			return nil
		case <-dp.scanTicker.C:
		}
	}
}

func (dp *DevicePlugin) scan() dpapi.DeviceTree {
	files, _ := filepath.Glob(dp.dlbDeviceFilePathReg)

	devTree := dpapi.NewDeviceTree()

	for _, file := range files {
		devs := []pluginapi.DeviceSpec{{
			HostPath:      file,
			ContainerPath: file,
			Permissions:   "rw",
		}}
		deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, devs, nil, nil)

		sysfsDev := filepath.Join(dp.sysfsDir, filepath.Base(file))
		sriovNumVFs := pluginutils.GetSriovNumVFs(sysfsDev)

		switch sriovNumVFs {
		case "0":
			devTree.AddDevice(deviceTypePF, file, deviceInfo)
		case "-1":
			devTree.AddDevice(deviceTypeVF, file, deviceInfo)
		default:
			continue
		}
	}

	return devTree
}

func main() {
	flag.Parse()
	klog.V(1).Infof("DLB device plugin started")

	plugin := NewDevicePlugin(dlbDeviceFilePathRE, sysfsDir)
	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
