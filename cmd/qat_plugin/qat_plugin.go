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
	"flag"
	"fmt"
	"os"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/dpdkdrv"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"k8s.io/klog/v2"
)

const (
	namespace = "qat.intel.com"
)

func main() {
	dpdkDriver := flag.String("dpdk-driver", "vfio-pci", "DPDK Device driver for configuring the QAT device")
	kernelVfDrivers := flag.String("kernel-vf-drivers", "4xxxvf,420xxvf", "Comma separated VF Device Driver of the QuickAssist Devices in the system. Devices supported: DH895xCC, C62x, C3xxx, C4xxx, 4xxx, 420xxx, 6xxx, and D15xx")
	preferredAllocationPolicy := flag.String("allocation-policy", "", "Modes of allocating QAT devices: balanced and packed")
	maxNumDevices := flag.Int("max-num-devices", 64, "maximum number of QAT devices to be provided to the QuickAssist device plugin")
	flag.Parse()

	plugin, err := dpdkdrv.NewDevicePlugin(*maxNumDevices, *kernelVfDrivers, *dpdkDriver, *preferredAllocationPolicy)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	klog.V(1).Infof("QAT device plugin started")

	manager := deviceplugin.NewManager(namespace, plugin)

	manager.Run()
}
