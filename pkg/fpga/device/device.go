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

package device

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	sysfsDirectoryOPAE      = "/sys/class/fpga"
	interfaceIDTemplateOPAE = "/sys/class/fpga/%s/*/pr/interface_id"
	afuIDTemplateOPAE       = "/sys/class/fpga/intel-fpga-dev.*/%s/afu_id"
	fpgaPortDevRegexOPAE    = `^intel-fpga-port\.[0-9]+$`
	devTemplateOPAE         = "/sys/class/fpga/intel-fpga-dev.*/%s/dev"

	sysfsDirectoryDFL      = "/sys/class/fpga_region"
	interfaceIDTemplateDFL = "/sys/class/fpga_region/%s/*/dfl-fme-region.*/fpga_region/region*/compat_id"
	afuIDTemplateDFL       = "/sys/class/fpga_region/region*/%s/afu_id"
	devTemplateDFL         = "/sys/class/fpga_region/region*/%s/dev"
	fpgaPortDevRegexDFL    = `^dfl-port\.[0-9]+$`

	fpgaDevRegex     = `.*/sys/class/fpga[^/]*/([^/]+)/.+$`
	fpgaPortDevRegex = `.*/sys/class/fpga[^/]*/[^/]+/([^/]+)/.+$`
)

var (
	fpgaPortDevRegDFL  = regexp.MustCompile(fpgaPortDevRegexDFL)
	fpgaPortDevRegOPAE = regexp.MustCompile(fpgaPortDevRegexOPAE)
	fpgaDevReg         = regexp.MustCompile(fpgaDevRegex)
	fpgaPortDevReg     = regexp.MustCompile(fpgaPortDevRegex)
)

// FPGADevice represents FME and port(AFU) devices
type FPGADevice struct {
	ID        string // Interface or AFU id
	Name      string // basename of DevNode
	SysfsPath string // path to ID file
	DevNode   string
	Minor     uint32
	Major     uint32
}

// CanonizeID canonizes Interface and AFU ids
func CanonizeID(ID string) string {
	return strings.ToLower(strings.Replace(strings.TrimSpace(ID), "-", "", -1))
}

// IsFPGADevice returns true if device is either DFL or OPAE device
func IsFPGADevice(deviceName string) bool {
	return fpgaPortDevRegDFL.MatchString(deviceName) || fpgaPortDevRegOPAE.MatchString(deviceName)
}

func getTemplates(sysFsPrefix string) (string, string, string, error) {
	var afuIDTemplate, interfaceIDTemplate, devTemplate string
	if _, err := os.Stat(path.Join(sysFsPrefix, sysfsDirectoryOPAE)); err == nil {
		afuIDTemplate = afuIDTemplateOPAE
		interfaceIDTemplate = interfaceIDTemplateOPAE
		devTemplate = devTemplateOPAE
	} else if _, err := os.Stat(path.Join(sysFsPrefix, sysfsDirectoryDFL)); err == nil {
		afuIDTemplate = afuIDTemplateDFL
		interfaceIDTemplate = interfaceIDTemplateDFL
		devTemplate = devTemplateDFL
	} else {
		return "", "", "", fmt.Errorf("kernel driver is not loaded: neither %s nor %s sysfs directory is accessible", sysfsDirectoryOPAE, sysfsDirectoryDFL)
	}
	return path.Join(sysFsPrefix, afuIDTemplate), path.Join(sysFsPrefix, interfaceIDTemplate), path.Join(sysFsPrefix, devTemplate), nil
}

// read file by filepath.Glob pattern
func readFileByPattern(pattern string) (string, []byte, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", nil, err
	}
	if len(matches) == 0 {
		return "", nil, fmt.Errorf("no file found with pattern '%s'", pattern)
	}
	if len(matches) > 1 {
		return "", nil, fmt.Errorf("path pattern '%s' matches multiple files", pattern)
	}
	data, err := ioutil.ReadFile(matches[0])
	if err != nil {
		return matches[0], nil, errors.WithStack(err)
	}
	return matches[0], data, nil
}

func parseDev(devData string) (uint32, uint32, error) {
	devData = strings.TrimSpace(devData)
	numbers := strings.SplitN(devData, ":", 2)
	minor := numbers[0]
	major := numbers[1]
	minorInt, err := strconv.ParseInt(minor, 10, 32)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "can't convert device minor %s to a number", minor)
	}
	majorInt, err := strconv.ParseInt(major, 10, 32)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "can't convert device major %s to a number", major)
	}

	return uint32(minorInt), uint32(majorInt), nil
}

func getFPGADevice(afuIDPattern, devPattern string) (*FPGADevice, error) {
	idPath, idData, err := readFileByPattern(afuIDPattern)
	if err != nil {
		return nil, err
	}

	// Get minor:major from the dev file
	_, devData, err := readFileByPattern(devPattern)
	if err != nil {
		return nil, err
	}

	major, minor, err := parseDev(string(devData))
	if err != nil {
		return nil, err
	}

	// Find fpga device name
	subs := fpgaPortDevReg.FindStringSubmatch(idPath)
	if len(subs) < 2 {
		return nil, fmt.Errorf("can't parse afu_id path: %s", idPath)
	}
	name := subs[1]

	return &FPGADevice{
		ID:        CanonizeID(string(idData)),
		Name:      name,
		SysfsPath: idPath,
		DevNode:   path.Join("/dev", name),
		Minor:     minor,
		Major:     major,
	}, nil
}

// GetAFUDevice creates FPGADevice struct for using port device name
func GetAFUDevice(sysFsPrefix, portDeviceName string) (*FPGADevice, error) {
	afuIDTemplate, _, devTemplate, err := getTemplates(sysFsPrefix)
	if err != nil {
		return nil, err
	}
	return getFPGADevice(fmt.Sprintf(afuIDTemplate, portDeviceName), fmt.Sprintf(devTemplate, portDeviceName))
}

// GetFMEDevice reads FME properties from SysFs
func GetFMEDevice(sysFsPrefix, portDeviceName string) (*FPGADevice, error) {
	portDeviceName = strings.TrimPrefix(portDeviceName, "/dev/")
	afuDevice, err := GetAFUDevice(sysFsPrefix, portDeviceName)
	if err != nil {
		return nil, err
	}
	// Get top level region name from the AFU path
	subs := fpgaDevReg.FindStringSubmatch(afuDevice.SysfsPath)
	if len(subs) < 2 {
		return nil, fmt.Errorf("can't parse sysfs path: %s", afuDevice.SysfsPath)
	}
	region := subs[1]

	_, interfaceIDTemplate, devTemplate, err := getTemplates(sysFsPrefix)
	if err != nil {
		return nil, err
	}

	return getFPGADevice(fmt.Sprintf(interfaceIDTemplate, region), fmt.Sprintf(devTemplate, portDeviceName))
}
