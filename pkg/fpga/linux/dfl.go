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
	"os"
	"unsafe"

	"github.com/pkg/errors"
)

// DflFME represent DFL FPGA FME device
type DflFME struct {
	FpgaFME
	DevPath string
	f       *os.File
}

// Close closes open device
func (f *DflFME) Close() error {
	if f.f != nil {
		return f.f.Close()
	}
	return nil
}

// NewDflFME Opens device
func NewDflFME(dev string) (FpgaFME, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	fme := &DflFME{DevPath: dev, f: f}
	// check that kernel API is compatible
	if _, err := fme.GetAPIVersion(); err != nil {
		fme.Close()
		return nil, errors.Wrap(err, "kernel API mismatch")
	}
	return fme, nil
}

// DflPort represent DFL FPGA Port device
type DflPort struct {
	FpgaPort
	DevPath string
	f       *os.File
}

// Close closes open device
func (f *DflPort) Close() error {
	if f.f != nil {
		return f.f.Close()
	}
	return nil
}

// NewDflPort Opens device
func NewDflPort(dev string) (FpgaPort, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	port := &DflPort{DevPath: dev, f: f}
	// check that kernel API is compatible
	if _, err := port.GetAPIVersion(); err != nil {
		port.Close()
		return nil, errors.Wrap(err, "kernel API mismatch")
	}
	return port, nil
}

// common ioctls for FME and Port
func commonGetAPIVersion(fd uintptr) (int, error) {
	v, err := ioctl(fd, DFL_FPGA_GET_API_VERSION, 0)
	return int(v), err
}
func commonCheckExtension(fd uintptr) (int, error) {
	v, err := ioctl(fd, DFL_FPGA_CHECK_EXTENSION, 0)
	return int(v), err
}

// GetAPIVersion  Report the version of the driver API.
// * Return: Driver API Version.
func (f *DflFME) GetAPIVersion() (int, error) {
	return commonGetAPIVersion(f.f.Fd())
}

// CheckExtension Check whether an extension is supported.
// * Return: 0 if not supported, otherwise the extension is supported.
func (f *DflFME) CheckExtension() (int, error) {
	return commonCheckExtension(f.f.Fd())
}

// GetAPIVersion  Report the version of the driver API.
// * Return: Driver API Version.
func (f *DflPort) GetAPIVersion() (int, error) {
	return commonGetAPIVersion(f.f.Fd())
}

// CheckExtension Check whether an extension is supported.
// * Return: 0 if not supported, otherwise the extension is supported.
func (f *DflPort) CheckExtension() (int, error) {
	return commonCheckExtension(f.f.Fd())
}

// PortReset Reset the FPGA Port and its AFU. No parameters are supported.
// Userspace can do Port reset at any time, e.g. during DMA or PR. But
// it should never cause any system level issue, only functional failure
// (e.g. DMA or PR operation failure) and be recoverable from the failure.
// * Return: 0 on success, -errno of failure
func (f *DflPort) PortReset() error {
	_, err := ioctl(f.f.Fd(), DFL_FPGA_PORT_RESET, 0)
	return err
}

// PortGetInfo Retrieve information about the fpga port.
// Driver fills the info in provided struct dfl_fpga_port_info.
// * Return: 0 on success, -errno on failure.
func (f *DflPort) PortGetInfo() (ret FpgaPortInfo, err error) {
	var value DflFpgaPortInfo
	value.Argsz = uint32(unsafe.Sizeof(value))
	_, err = ioctl(f.f.Fd(), DFL_FPGA_PORT_GET_INFO, uintptr(unsafe.Pointer(&value)))
	if err == nil {
		ret.Flags = value.Flags
		ret.Regions = value.Regions
		ret.Umsgs = value.Umsgs
	}
	return
}

// PortGetRegionInfo Retrieve information about the fpga port.
// * Retrieve information about a device memory region.
// * Caller provides struct dfl_fpga_port_region_info with index value set.
// * Driver returns the region info in other fields.
// * Return: 0 on success, -errno on failure.
func (f *DflPort) PortGetRegionInfo(index uint32) (ret FpgaPortRegionInfo, err error) {
	var value DflFpgaPortRegionInfo
	value.Argsz = uint32(unsafe.Sizeof(value))
	value.Index = index
	_, err = ioctl(f.f.Fd(), DFL_FPGA_PORT_GET_REGION_INFO, uintptr(unsafe.Pointer(&value)))
	if err == nil {
		ret.Flags = value.Flags
		ret.Index = value.Index
		ret.Offset = value.Offset
		ret.Size = value.Size
	}
	return
}

// PortPR does Partial Reconfiguration based on Port ID and Buffer (Image)
// provided by caller.
// * Return: 0 on success, -errno on failure.
// * If DFL_FPGA_FME_PORT_PR returns -EIO, that indicates the HW has detected
//   some errors during PR, under this case, the user can fetch HW error info
//   from the status of FME's fpga manager.
func (f *DflFME) PortPR(port uint32, bitstream []byte) error {
	var value DflFpgaFmePortPR
	value.Argsz = uint32(unsafe.Sizeof(value))
	value.Port_id = port
	value.Buffer_size = uint32(len(bitstream))
	value.Buffer_address = uint64(uintptr(unsafe.Pointer(&bitstream[0])))
	_, err := ioctl(f.f.Fd(), DFL_FPGA_FME_PORT_PR, uintptr(unsafe.Pointer(&value)))
	return err
}
