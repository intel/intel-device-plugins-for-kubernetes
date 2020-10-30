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

package main

import (
	"regexp"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga"
)

const (
	opaeDeviceRE = `^intel-fpga-dev.[0-9]+$`
	opaePortRE   = `^intel-fpga-port.[0-9]+$`
)

// newDevicePlugin returns new instance of devicePlugin.
func newDevicePluginOPAE(sysfsDir string, devfsDir string, mode string) (*devicePlugin, error) {
	getDevTree, annotationValue, err := getPluginParams(mode)
	if err != nil {
		return nil, err
	}

	return &devicePlugin{
		name: "OPAE",

		sysfsDir: sysfsDir,
		devfsDir: devfsDir,

		deviceReg: regexp.MustCompile(opaeDeviceRE),
		portReg:   regexp.MustCompile(opaePortRE),

		getDevTree: getDevTree,
		newPort:    fpga.NewIntelFpgaPort,

		annotationValue: annotationValue,
	}, nil
}
