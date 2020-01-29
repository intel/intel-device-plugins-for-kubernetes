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

package fpga

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/bitstream"

	"github.com/pkg/errors"
)

// IsFpgaFME returns true if the name looks like any supported FME device
func IsFpgaFME(name string) bool {
	devName := cleanBasename(name)
	return strings.HasPrefix(devName, dflFpgaFmePrefix) || strings.HasPrefix(devName, intelFpgaFmePrefix)
}

// IsFpgaPort returns true if the name looks like any supported FME device
func IsFpgaPort(name string) bool {
	devName := cleanBasename(name)
	return strings.HasPrefix(devName, dflFpgaPortPrefix) || strings.HasPrefix(devName, intelFpgaPortPrefix)
}

// CanonizeID canonizes Interface and AFU ids
func CanonizeID(ID string) string {
	return strings.ToLower(strings.Replace(strings.TrimSpace(ID), "-", "", -1))
}

// NewFpgaPort returns FpgaPort for specified device node
func NewFpgaPort(fname string) (FpgaPort, error) {
	if strings.IndexByte(fname, byte('/')) < 0 {
		fname = filepath.Join("/dev", fname)
	}
	devName := cleanBasename(fname)
	switch {
	case strings.HasPrefix(devName, dflFpgaPortPrefix):
		return NewDflPort(fname)
	case strings.HasPrefix(devName, intelFpgaPortPrefix):
		return NewIntelFpgaPort(fname)
	}
	return nil, errors.Errorf("unknown type of FPGA port %s", fname)
}

// NewFpgaFME returns FpgaFME for specified device node
func NewFpgaFME(fname string) (FpgaFME, error) {
	if strings.IndexByte(fname, byte('/')) < 0 {
		fname = filepath.Join("/dev", fname)
	}
	devName := cleanBasename(fname)
	switch {
	case strings.HasPrefix(devName, dflFpgaFmePrefix):
		return NewDflFME(fname)
	case strings.HasPrefix(devName, intelFpgaFmePrefix):
		return NewIntelFpgaFME(fname)
	}
	return nil, errors.Errorf("unknown type of FPGA FME %s", fname)
}

// ListFpgaDevices returns two lists of FPGA device nodes: FMEs and Ports
func ListFpgaDevices() (FMEs, Ports []string) {
	files, err := ioutil.ReadDir("/sys/bus/platform/devices")
	if err != nil {
		return
	}
	for _, file := range files {
		fname := file.Name()
		switch {
		case IsFpgaFME(fname):
			FMEs = append(FMEs, fname)
		case IsFpgaPort(fname):
			Ports = append(Ports, fname)
		}
	}
	return
}

func genericPortPR(f FpgaPort, bs bitstream.File, dryRun bool) error {
	fme, err := f.GetFME()
	if err != nil {
		return err
	}
	ifID := fme.GetInterfaceUUID()
	bsID := bs.InterfaceUUID()
	if ifID != bsID {
		return errors.Errorf("FME interface UUID %q is not compatible with bitstream UUID %q ", ifID, bsID)
	}
	pNum, err := f.GetPortID()
	if err != nil {
		return err
	}
	rawBistream, err := bs.RawBitstreamData()
	if err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	return fme.PortPR(pNum, rawBistream)
}
