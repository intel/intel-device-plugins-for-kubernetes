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
	"os"
	"path"
	"regexp"

	"github.com/pkg/errors"
)

const (
	opaeDeviceRE = `^intel-fpga-dev.[0-9]+$`
	opaePortRE   = `^intel-fpga-port.[0-9]+$`
	opaeFmeRE    = `^intel-fpga-fme.[0-9]+$`
)

func getSysFsInfoOPAE(dp *devicePlugin, deviceFolder string, deviceFiles []os.FileInfo, fname string) ([]region, []afu, error) {
	var regions []region
	var afus []afu
	for _, deviceFile := range deviceFiles {
		name := deviceFile.Name()

		if dp.fmeReg.MatchString(name) {
			if len(regions) > 0 {
				return nil, nil, errors.Errorf("Detected more than one FPGA region for device %s. Only one region per FPGA device is supported", fname)
			}
			interfaceIDFile := path.Join(deviceFolder, name, "pr", "interface_id")
			region, err := dp.getFME(interfaceIDFile, name)
			if err != nil {
				return nil, nil, err
			}
			regions = append(regions, *region)
		} else if dp.portReg.MatchString(name) {
			afuPath := path.Join(deviceFolder, name, "afu_id")
			afu, err := dp.getAFU(afuPath, name)
			if err != nil {
				return nil, nil, err
			}
			afus = append(afus, *afu)
		}
	}

	return regions, afus, nil
}

// newDevicePlugin returns new instance of devicePlugin
func newDevicePluginOPAE(sysfsDir string, devfsDir string, mode string) (*devicePlugin, error) {
	getDevTree, ignoreAfuIDs, annotationValue, err := getPluginParams(mode)
	if err != nil {
		return nil, err
	}

	return &devicePlugin{
		name: "OPAE",

		sysfsDir: sysfsDir,
		devfsDir: devfsDir,

		deviceReg: regexp.MustCompile(opaeDeviceRE),
		portReg:   regexp.MustCompile(opaePortRE),
		fmeReg:    regexp.MustCompile(opaeFmeRE),

		getDevTree:   getDevTree,
		getSysFsInfo: getSysFsInfoOPAE,

		annotationValue: annotationValue,
		ignoreAfuIDs:    ignoreAfuIDs,
	}, nil
}
