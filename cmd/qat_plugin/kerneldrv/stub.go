// Copyright 2019 Intel Corporation. All Rights Reserved.
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

// +build !kerneldrv

package kerneldrv

import (
	"os"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"k8s.io/klog"
)

// NewDevicePlugin creates a non-functional stub for kernel mode device plugins.
func NewDevicePlugin() dpapi.Scanner {
	klog.Errorf("kernel mode is not supported in this build. Use 'kerneldrv' build tag to have this mode enabled. Exiting...")
	os.Exit(1)

	return nil
}
