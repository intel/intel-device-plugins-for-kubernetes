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

// IntelFpgaFME represent Intel FPGA FME device
type IntelFpgaFME struct {
	FpgaFME
	DevPath string
	f       *os.File
}

// Close closes open device
func (f *IntelFpgaFME) Close() error {
	if f.f != nil {
		return f.f.Close()
	}
	return nil
}

// NewIntelFpgaFME Opens device
func NewIntelFpgaFME(dev string) (FpgaFME, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	fme := &IntelFpgaFME{DevPath: dev, f: f}
	// check that kernel API is compatible
	if _, err := fme.GetAPIVersion(); err != nil {
		fme.Close()
		return nil, errors.Wrap(err, "kernel API mismatch")
	}
	return fme, nil
}

// IntelFpgaPort represent IntelFpga FPGA Port device
type IntelFpgaPort struct {
	FpgaPort
	DevPath string
	f       *os.File
}

// Close closes open device
func (f *IntelFpgaPort) Close() error {
	if f.f != nil {
		return f.f.Close()
	}
	return nil
}

// NewIntelFpgaPort Opens device
func NewIntelFpgaPort(dev string) (FpgaPort, error) {
	f, err := os.OpenFile(dev, os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	port := &IntelFpgaPort{DevPath: dev, f: f}
	// check that kernel API is compatible
	if _, err := port.GetAPIVersion(); err != nil {
		port.Close()
		return nil, errors.Wrap(err, "kernel API mismatch")
	}
	return port, nil
}

// common ioctls for FME and Port
func commonIntelFpgaGetAPIVersion(fd uintptr) (int, error) {
	v, err := ioctl(fd, FPGA_GET_API_VERSION, 0)
	return int(v), err
}
func commonIntelFpgaCheckExtension(fd uintptr) (int, error) {
	v, err := ioctl(fd, FPGA_CHECK_EXTENSION, 0)
	return int(v), err
}

// GetAPIVersion  Report the version of the driver API.
// * Return: Driver API Version.
func (f *IntelFpgaFME) GetAPIVersion() (int, error) {
	return commonIntelFpgaGetAPIVersion(f.f.Fd())
}

// CheckExtension Check whether an extension is supported.
// * Return: 0 if not supported, otherwise the extension is supported.
func (f *IntelFpgaFME) CheckExtension() (int, error) {
	return commonIntelFpgaCheckExtension(f.f.Fd())
}

// GetAPIVersion  Report the version of the driver API.
// * Return: Driver API Version.
func (f *IntelFpgaPort) GetAPIVersion() (int, error) {
	return commonIntelFpgaGetAPIVersion(f.f.Fd())
}

// CheckExtension Check whether an extension is supported.
// * Return: 0 if not supported, otherwise the extension is supported.
func (f *IntelFpgaPort) CheckExtension() (int, error) {
	return commonIntelFpgaCheckExtension(f.f.Fd())
}

// PortReset Reset the FPGA Port and its AFU. No parameters are supported.
// Userspace can do Port reset at any time, e.g. during DMA or PR. But
// it should never cause any system level issue, only functional failure
// (e.g. DMA or PR operation failure) and be recoverable from the failure.
// * Return: 0 on success, -errno of failure
func (f *IntelFpgaPort) PortReset() error {
	_, err := ioctl(f.f.Fd(), FPGA_PORT_RESET, 0)
	return err
}

// PortGetInfo Retrieve information about the fpga port.
// Driver fills the info in provided struct IntelFpga_fpga_port_info.
// * Return: 0 on success, -errno on failure.
func (f *IntelFpgaPort) PortGetInfo() (ret FpgaPortInfo, err error) {
	var value IntelFpgaPortInfo
	value.Argsz = uint32(unsafe.Sizeof(value))
	_, err = ioctl(f.f.Fd(), FPGA_PORT_GET_INFO, uintptr(unsafe.Pointer(&value)))
	if err == nil {
		ret.Flags = value.Flags
		ret.Regions = value.Regions
		ret.Umsgs = value.Umsgs
	}
	return
}

// PortGetRegionInfo Retrieve information about the fpga port.
// * Retrieve information about a device memory region.
// * Caller provides struct IntelFpga_fpga_port_region_info with index value set.
// * Driver returns the region info in other fields.
// * Return: 0 on success, -errno on failure.
func (f *IntelFpgaPort) PortGetRegionInfo(index uint32) (ret FpgaPortRegionInfo, err error) {
	var value IntelFpgaPortRegionInfo
	value.Argsz = uint32(unsafe.Sizeof(value))
	value.Index = index
	_, err = ioctl(f.f.Fd(), FPGA_PORT_GET_REGION_INFO, uintptr(unsafe.Pointer(&value)))
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
// * If IntelFpga_FPGA_FME_PORT_PR returns -EIO, that indicates the HW has detected
//   some errors during PR, under this case, the user can fetch HW error info
//   from the status of FME's fpga manager.
func (f *IntelFpgaFME) PortPR(port uint32, bitstream []byte) error {
	var value IntelFpgaFmePortPR
	value.Argsz = uint32(unsafe.Sizeof(value))
	value.Port_id = port
	value.Buffer_size = uint32(len(bitstream))
	value.Buffer_address = uint64(uintptr(unsafe.Pointer(&bitstream[0])))
	_, err := ioctl(f.f.Fd(), FPGA_FME_PORT_PR, uintptr(unsafe.Pointer(&value)))
	return err
}
