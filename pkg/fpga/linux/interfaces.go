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

import "io"

type commonAPI interface {
	io.Closer
	// GetAPIVersion  Report the version of the driver API.
	// * Return: Driver API Version.
	GetAPIVersion() (int, error)
	// CheckExtension Check whether an extension is supported.
	// * Return: 0 if not supported, otherwise the extension is supported.
	CheckExtension() (int, error)
}

// FpgaFME represent interfaces provided by management interface of FPGA
type FpgaFME interface {
	commonAPI
	// PortPR does Partial Reconfiguration based on Port ID and Buffer (Image)
	// provided by caller.
	// * Return: 0 on success, -errno on failure.
	// * If DFL_FPGA_FME_PORT_PR returns -EIO, that indicates the HW has detected
	//   some errors during PR, under this case, the user can fetch HW error info
	//   from the status of FME's fpga manager.
	PortPR(uint32, []byte) error

	// TODO:
	// Port release / assign
	// Get Info
	// Set IRQ err
}

// FpgaPort represent interfaces provided by AFU port of FPGA
type FpgaPort interface {
	commonAPI
	// PortReset Reset the FPGA Port and its AFU. No parameters are supported.
	// Userspace can do Port reset at any time, e.g. during DMA or PR. But
	// it should never cause any system level issue, only functional failure
	// (e.g. DMA or PR operation failure) and be recoverable from the failure.
	// * Return: 0 on success, -errno of failure
	PortReset() error
	// PortGetInfo Retrieve information about the fpga port.
	// Driver fills the info in provided struct dfl_fpga_port_info.
	// * Return: 0 on success, -errno on failure.
	PortGetInfo() (FpgaPortInfo, error)
	// PortGetRegionInfo Retrieve information about the fpga port.
	// * Retrieve information about a device memory region.
	// * Caller provides struct dfl_fpga_port_region_info with index value set.
	// * Driver returns the region info in other fields.
	// * Return: 0 on success, -errno on failure.
	PortGetRegionInfo(index uint32) (FpgaPortRegionInfo, error)

	// TODO:
	// Port DMA map / unmap
	// UMSG enable / disable / set-mode / set-base-addr (intel-fpga)
	// Set IRQ: err, uafu (intel-fpga)
}

// FpgaPortInfo is a unified port info between drivers
type FpgaPortInfo struct {
	Flags   uint32
	Regions uint32
	Umsgs   uint32
}

// FpgaPortRegionInfo is a unified Port Region info between drivers
type FpgaPortRegionInfo struct {
	Flags  uint32
	Index  uint32
	Size   uint64
	Offset uint64
}
