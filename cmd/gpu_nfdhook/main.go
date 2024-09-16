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
	"os"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/labeler"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fakedri"
)

var (
	sysfsDirectory    = "/host-sys"
	sysfsDRMDirectory = sysfsDirectory + "/class/drm"
)

func main() {
	fakedriSpec := os.Getenv("FAKEDRI_SPEC")
	// Check if fakedriSpec is empty, and if so use system sysfs
	if fakedriSpec != "" {
		options := fakedri.GetOptionsBySpec(fakedriSpec)
		fakedri.GenerateDriFiles(options)

		sysfsDirectory = "sys"
		sysfsDRMDirectory = sysfsDirectory + "/class/drm"
	}

	labeler.CreateAndPrintLabels(sysfsDRMDirectory)
}
