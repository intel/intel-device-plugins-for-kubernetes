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
//
// +build linux
//

package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	pciAddressRegex = `^([[:xdigit:]]{4}):([[:xdigit:]]{2}):([[:xdigit:]]{2})\.([[:xdigit:]])$`
	vendorIntel     = "0x8086"
	fpgaClass       = "0x120000"
)

var (
	pciAddressRE = regexp.MustCompile(pciAddressRegex)
)

// PCIDevice represents most valuable sysfs information about PCI device
type PCIDevice struct {
	SysFsPath string
	BDF       string
	Vendor    string
	Device    string
	Class     string
	CPUs      string
	NUMA      string
	VFs       string
	TotalVFs  string
	PhysFn    *PCIDevice
}

// NewPCIDevice returns sysfs entry for specified PCI device
func NewPCIDevice(devPath string) (*PCIDevice, error) {
	realDevPath, err := filepath.EvalSymlinks(devPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed get realpath for %s", devPath)
	}
	pci := new(PCIDevice)
	for p := realDevPath; strings.HasPrefix(p, "/sys/devices/pci"); p = filepath.Dir(p) {
		subs := pciAddressRE.FindStringSubmatch(filepath.Base(p))
		if subs == nil || len(subs) != 5 {
			continue
		}
		pci.SysFsPath = p
		pci.BDF = subs[0]
		break
	}
	if pci.SysFsPath == "" || pci.BDF == "" {
		return nil, errors.Errorf("can't find PCI device address for sysfs entry %s", realDevPath)
	}
	fileMap := map[string]*string{
		"vendor":         &pci.Vendor,
		"device":         &pci.Device,
		"class":          &pci.Class,
		"local_cpulist":  &pci.CPUs,
		"numa_node":      &pci.NUMA,
		"sriov_numvfs":   &pci.VFs,
		"sriov_totalvfs": &pci.TotalVFs,
	}
	if err = readFilesInDirectory(fileMap, pci.SysFsPath); err != nil {
		return nil, err
	}
	if pci.Vendor == "" || pci.Device == "" {
		return nil, errors.Errorf("%s vendor or device id can't be empty (%q/%q)", pci.SysFsPath, pci.Vendor, pci.Device)
	}
	if physFn, err := NewPCIDevice(filepath.Join(pci.SysFsPath, "physfn")); err == nil {
		pci.PhysFn = physFn
	}
	return pci, nil
}

// NumVFs returns number of configured VFs
func (pci *PCIDevice) NumVFs() int64 {
	if numvfs, err := strconv.ParseInt(pci.VFs, 10, 32); err == nil {
		return numvfs
	}
	return -1
}

// GetVFs returns array of PCI device sysfs entries for VFs
func (pci *PCIDevice) GetVFs() (ret []*PCIDevice, err error) {
	if pci.NumVFs() > 0 {
		dirs, _ := filepath.Glob(filepath.Join(pci.SysFsPath, "virtfn*"))
		for _, dir := range dirs {
			vf, er := NewPCIDevice(dir)
			if er != nil {
				return nil, er
			}
			ret = append(ret, vf)
		}
	}
	return
}

// FindSysFsDevice returns sysfs entry for specified device node or device that holds specified file
// If resulted device is virtual, error is returned
func FindSysFsDevice(dev string) (string, error) {
	fi, err := os.Stat(dev)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", errors.Wrapf(err, "unable to get stat for %s", dev)
	}

	devType := "block"
	rdev := fi.Sys().(*syscall.Stat_t).Dev
	if mode := fi.Mode(); mode&os.ModeDevice != 0 {
		rdev = fi.Sys().(*syscall.Stat_t).Rdev
		if mode&os.ModeCharDevice != 0 {
			devType = "char"
		}
	}

	major := unix.Major(rdev)
	minor := unix.Minor(rdev)
	if major == 0 {
		return "", errors.Errorf("%s is a virtual device node", dev)
	}
	devPath := fmt.Sprintf("/sys/dev/%s/%d:%d", devType, major, minor)
	realDevPath, err := filepath.EvalSymlinks(devPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed get realpath for %s", devPath)
	}
	return realDevPath, nil
}
