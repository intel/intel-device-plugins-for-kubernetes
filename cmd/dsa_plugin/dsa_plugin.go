// Copyright 2020-2026 Intel Corporation. All Rights Reserved.
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
	"os"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/idxd"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/vfio"

	"k8s.io/klog/v2"
)

const (
	// Device plugin settings.
	namespace = "dsa.intel.com"
	// Device directories.
	devDir = "/dev/dsa"
	// Glob pattern for the state sysfs entry.
	statePattern = "/sys/bus/dsa/devices/dsa*/wq*/state"

	pciDevicesDir = "/sys/bus/pci/devices"
)

func main() {
	var (
		sharedDevNum int
		plugin       dpapi.Scanner
	)

	flag.IntVar(&sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same work queue")
	dsaDriver := flag.String("driver", "idxd", "Device driver used for the DSA devices")
	flag.Parse()

	if sharedDevNum < 1 {
		klog.Warning("The number of containers sharing the same work queue must be greater than zero.")
		os.Exit(1)
	}

	switch *dsaDriver {
	case "idxd":
		plugin = idxd.NewDevicePlugin(statePattern, devDir, sharedDevNum)
	case "vfio-pci":
		dsaDeviceIDs := vfio.DeviceIDSet{
			"0x0b25": {},
			"0x11fb": {},
			"0x1212": {},
		}
		if sharedDevNum > 1 {
			klog.Warning("shared-dev-num setting ignored when using -driver=vfio-pci.")
		}
		plugin = vfio.NewDevicePlugin(pciDevicesDir, dsaDeviceIDs)
	default:
		klog.Warningf("Unsupported DSA driver: %s. Use either idxd or vfio-pci.", *dsaDriver)
		os.Exit(1)
	}

	manager := dpapi.NewManager(namespace, plugin)

	manager.Run()
}
