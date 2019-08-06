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
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/pkg/errors"
)

const (
	dflDeviceRE = `^region[0-9]+$`
	dflPortRE   = `^dfl-port\.[0-9]+$`
	dflFmeRE    = `^dfl-fme\.[0-9]+$`
)

// getDFLRegion reads FME interface id from /sys/fpga/fpga_region/regionX/dfl-fme.k/dfl-fme-region.n/fpga_region/regionN/compat_id
func (dp *devicePlugin) getDFLRegion(regionFolder string, fme string) (*region, error) {
	compatIDPattern := path.Join(regionFolder, fme, "dfl-fme-region.*", "fpga_region", "region*", "compat_id")
	matches, err := filepath.Glob(compatIDPattern)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no compat_id found with pattern '%s'", compatIDPattern)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("compat_id path pattern '%s' matches multiple files", compatIDPattern)
	}

	reg, err := dp.getFME(matches[0], fme)
	if err != nil {
		return nil, err
	}

	return reg, nil
}

func getSysFsInfoDFL(dp *devicePlugin, deviceFolder string, deviceFiles []os.FileInfo, fname string) ([]region, []afu, error) {
	var regions []region
	var afus []afu

	for _, deviceFile := range deviceFiles {
		name := deviceFile.Name()

		if dp.fmeReg.MatchString(name) {
			if len(regions) > 0 {
				return nil, nil, errors.Errorf("Detected more than one FPGA region for device %s. Only one region per FPGA device is supported", fname)
			}

			region, err := dp.getDFLRegion(deviceFolder, name)
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

// newDevicePluginDFL returns new instance of devicePlugin
func newDevicePluginDFL(sysfsDir string, devfsDir string, mode string) (*devicePlugin, error) {
	getDevTree, ignoreAfuIDs, annotationValue, err := getPluginParams(mode)
	if err != nil {
		return nil, err
	}

	return &devicePlugin{
		name: "DFL",

		sysfsDir: sysfsDir,
		devfsDir: devfsDir,

		deviceReg: regexp.MustCompile(dflDeviceRE),
		portReg:   regexp.MustCompile(dflPortRE),
		fmeReg:    regexp.MustCompile(dflFmeRE),

		getDevTree:   getDevTree,
		getSysFsInfo: getSysFsInfoDFL,

		ignoreEmptyRegions: true,
		ignoreAfuIDs:       ignoreAfuIDs,
		annotationValue:    annotationValue,
	}, nil
}
