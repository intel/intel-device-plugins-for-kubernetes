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

package bitstream

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	utilsexec "k8s.io/utils/exec"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/device"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/linux"
	"github.com/pkg/errors"
)

const (
	fpgaconf        = "/opt/intel/fpga-sw/opae/fpgaconf-wrapper"
	pciAddressRegex = `^[[:xdigit:]]{4}:([[:xdigit:]]{2}):([[:xdigit:]]{2})\.([[:xdigit:]])$`
)

var (
	pciAddressRE = regexp.MustCompile(pciAddressRegex)
)

// FPGABitstream defines common interface for OPAE and OpenCL bitstreams
type FPGABitstream interface {
	Init() error
	Validate(region, afu string) error
	Program(fme *device.FPGADevice, execer utilsexec.Interface) error
}

// OPAEBitstream defines structure of .gbs file
type OPAEBitstream struct {
	Path   string
	Region string
	AFU    string
}

// Init gets Region and AFU from .gbs file
func (bitstream *OPAEBitstream) Init() error {
	gbs, err := OpenGBS(bitstream.Path)
	if err != nil {
		return errors.Wrapf(err, "%s: can't get bitstream info", bitstream.Path)
	}

	if len(gbs.Metadata.AfuImage.AcceleratorClusters) != 1 {
		return errors.Errorf("%s: 'accelerator-clusters' field not found", bitstream.Path)
	}

	bitstream.Region = device.CanonizeID(gbs.Metadata.AfuImage.InterfaceUUID)
	bitstream.AFU = device.CanonizeID(gbs.Metadata.AfuImage.AcceleratorClusters[0].AcceleratorTypeUUID)

	return nil
}

// GetFPGABitstream scans bitstream storage and returns first found bitstream by region and afu id
func GetFPGABitstream(bitstreamDir, region, afu string) (FPGABitstream, error) {
	bitstreamPath := ""
	// Temporarily only support gbs bitstreams
	// for _, ext := range []string{".gbs", ".aocx"} {
	for _, ext := range []string{".gbs", ".aocx"} {
		bitstreamPath = filepath.Join(bitstreamDir, region, afu+ext)

		_, err := os.Stat(bitstreamPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.Errorf("%s: stat error: %v", bitstreamPath, err)
		}

		if ext == ".gbs" {
			return &OPAEBitstream{
				Path:   bitstreamPath,
				Region: region,
				AFU:    afu}, nil
		} else if ext == ".aocx" {
			return &OpenCLBitstream{
				Path:   bitstreamPath,
				Region: region,
				AFU:    afu}, nil
		}
	}
	return nil, errors.Errorf("%s/%s: bitstream not found", region, afu)
}

func validate(region, afu, expectedRegion, expectedAFU string) error {
	if expectedRegion != region {
		return errors.Errorf("bitstream is not for this device: region(%s) and interface-uuid(%s) don't match", region, expectedRegion)
	}

	if expectedAFU != afu {
		return errors.Errorf("incorrect bitstream: AFU(%s) and accelerator-type-uuid(%s) don't match", afu, expectedAFU)
	}

	return nil
}

// Validate checks if region and afu parameters mutch bitstream parameters
func (bitstream *OPAEBitstream) Validate(region, afu string) error {
	return validate(bitstream.Region, bitstream.AFU, region, afu)
}

func getFpgaConfArgs(devNode string) ([]string, error) {
	realDevPath, err := linux.FindSysFsDevice(devNode)
	if err != nil {
		return nil, err
	}
	if realDevPath == "" {
		return nil, nil
	}
	for p := realDevPath; strings.HasPrefix(p, "/sys/devices/pci"); p = filepath.Dir(p) {
		pciDevPath, err := filepath.EvalSymlinks(filepath.Join(p, "device"))
		if err != nil {
			continue
		}
		subs := pciAddressRE.FindStringSubmatch(filepath.Base(pciDevPath))
		if subs == nil || len(subs) != 4 {
			return nil, errors.Errorf("unable to parse PCI address %s", pciDevPath)
		}
		return []string{"-B", "0x" + subs[1], "-D", "0x" + subs[2], "-F", "0x" + subs[3]}, nil
	}
	return nil, errors.Errorf("can't find PCI device address for sysfs entry %s", realDevPath)
}

// Program programs OPAE gbs bitstream
func (bitstream *OPAEBitstream) Program(device *device.FPGADevice, execer utilsexec.Interface) error {
	args, err := getFpgaConfArgs(device.DevNode)
	if err != nil {
		return errors.Wrapf(err, "failed get fpgaconf args for %s", device.DevNode)
	}
	args = append(args, bitstream.Path)
	output, err := execer.Command(fpgaconf, args...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to program AFU %s to device %s, region %s: output: %s", bitstream.AFU, device.DevNode, bitstream.Region, string(output))
	}
	return nil
}

// OpenCLBitstream defines parameters of .aocx file
type OpenCLBitstream struct {
	Path   string
	Region string
	AFU    string
}

// Init stub for OpenCLBitstream
func (bitstream *OpenCLBitstream) Init() error {
	// Unpack .gbs and call OPAEBitstream.init() here
	return nil
}

// Validate stub for OpenCLBitstream
func (bitstream *OpenCLBitstream) Validate(region, afu string) error {
	return validate(bitstream.Region, bitstream.AFU, region, afu)
}

// Program stub for OpenCLBitstream
func (bitstream *OpenCLBitstream) Program(device *device.FPGADevice, execer utilsexec.Interface) error {
	return fmt.Errorf("Not implemented")
}

// Open bitstream file, detecting type based on the filename extension.
func Open(fname string) (File, error) {
	switch filepath.Ext(fname) {
	case ".gbs":
		return OpenGBS(fname)
	case ".aocx":
		return OpenAOCX(fname)
	}
	return nil, errors.Errorf("unsupported file format %s", fname)
}
