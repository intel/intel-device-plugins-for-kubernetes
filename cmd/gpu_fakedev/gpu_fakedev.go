// Copyright 2021-2024 Intel Corporation. All Rights Reserved.
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

//---------------------------------------------------------------
// sysfs SPECIFICATION
//
// sys/class/drm/cardX/
// sys/class/drm/cardX/lmem_total_bytes (gpu memory size, number)
// sys/class/drm/cardX/device/
// sys/class/drm/cardX/device/vendor (0x8086)
// sys/class/drm/cardX/device/sriov_numvfs (PF only, number of VF GPUs, number)
// sys/class/drm/cardX/device/drm/
// sys/class/drm/cardX/device/drm/cardX/
// sys/class/drm/cardX/device/drm/renderD1XX/
// sys/class/drm/cardX/device/numa_node (Numa node index[1], number)
// [1] indexing these: /sys/devices/system/node/nodeX/
//---------------------------------------------------------------
// devfs SPECIFICATION
//
// dev/dri/cardX
// dev/dri/renderD1XX
//---------------------------------------------------------------

package main

import (
    "flag"
    "log"
    "github.com/intel/intel-device-plugins-for-kubernetes/pkg/fakedri"
)

func main() {
    var name string
    flag.StringVar(&name, "json", "", "JSON spec for fake device sysfs, debugfs and devfs content")
    flag.BoolVar(&fakedri.Verbose, "verbose", false, "More verbose output")
    flag.Parse()

    if name == "" {
        log.Fatal("ERROR: no fake device spec provided")
    }

    options := fakedri.GetOptions(name)
    fakedri.GenerateDriFiles(options)
}
