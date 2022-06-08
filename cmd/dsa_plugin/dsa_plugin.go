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
	"os"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/idxd"

	"k8s.io/klog/v2"
)

const (
	// Device plugin settings.
	namespace = "dsa.intel.com"
	// Device directories.
	devDir = "/dev/dsa"
	// Glob pattern for the state sysfs entry.
	statePattern = "/sys/bus/dsa/devices/dsa*/wq*/state"
)

func main() {
	var sharedDevNum int

	flag.IntVar(&sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same work queue")
	flag.Parse()

	if sharedDevNum < 1 {
		klog.Warning("The number of containers sharing the same work queue must be greater than zero")
		os.Exit(1)
	}

	plugin := idxd.NewDevicePlugin(statePattern, devDir, sharedDevNum)
	if plugin == nil {
		klog.Fatal("Cannot create device plugin, please check above error messages.")
	}

	manager := dpapi.NewManager(namespace, plugin)

	manager.Run()
}
