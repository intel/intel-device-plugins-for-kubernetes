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
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/bitstream"
	"io"
)

type commonFpgaAPI interface {
	// Generic interfaces provided by FPGA ports and FMEs
	io.Closer
	// GetAPIVersion  Report the version of the driver API.
	// * Return: Driver API Version.
	GetAPIVersion() (int, error)
	// CheckExtension Check whether an extension is supported.
	// * Return: 0 if not supported, otherwise the extension is supported.
	CheckExtension() (int, error)

	// Interfaces for device discovery and accessing properties

	// GetDevPath returns path to device node
	GetDevPath() string
	// GetSysFsPath returns sysfs entry for FPGA FME or Port (e.g. can be used for custom errors/perf items)
	GetSysFsPath() string
	// GetName returns simple FPGA name, derived from sysfs entry, can be used with /dev/ or /sys/bus/platform/
	GetName() string
	// GetPCIDevice returns PCIDevice for this device
	GetPCIDevice() (*PCIDevice, error)
}

// FpgaFME represent interfaces provided by management interface of FPGA
type FpgaFME interface {
	// Kernel IOCTL interfaces for FPGA ports:
	commonFpgaAPI
	// PortPR does Partial Reconfiguration based on Port ID and Buffer (Image)
	// provided by caller.
	// * Return: 0 on success, -errno on failure.
	// * If DFL_FPGA_FME_PORT_PR returns -EIO, that indicates the HW has detected
	//   some errors during PR, under this case, the user can fetch HW error info
	//   from the status of FME's fpga manager.
	PortPR(uint32, []byte) error
	// TODO: (not implemented IOCTLs)
	// Port release / assign
	// Get Info
	// Set IRQ err

	// Interfaces for device discovery and accessing properties

	// GetPortsNum returns amount of FPGA Ports associated to this FME
	GetPortsNum() int
	// InterfaceUUID returns Interface UUID for FME
	GetInterfaceUUID() string
	// GetSocketID returns physical socket number, in case NUMA enumeration fails
	GetSocketID() (uint32, error)
	// GetBitstreamID returns FME bitstream id
	GetBitstreamID() string
	// GetBitstreamMetadata returns FME bitstream metadata
	GetBitstreamMetadata() string
	// GetPort returns FpgaPort of the desired FPGA port index within that FME
	// GetPort(uint32) (FpgaPort, error)
}

// FpgaPort represent interfaces provided by AFU port of FPGA
type FpgaPort interface {
	// Kernel IOCTL interfaces for FPGA ports:
	commonFpgaAPI
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
	// TODO: (not implemented IOCTLs)
	// Port DMA map / unmap
	// UMSG enable / disable / set-mode / set-base-addr (intel-fpga)
	// Set IRQ: err, uafu (intel-fpga)

	// Interfaces for device discovery and accessing properties

	// GetFME returns FPGA FME device for this port
	GetFME() (FpgaFME, error)
	// GetPortID returns ID of the FPGA port within physical device
	GetPortID() (uint32, error)
	// GetAcceleratorTypeUUID returns AFU UUID for port
	GetAcceleratorTypeUUID() string
	// InterfaceUUID returns Interface UUID for FME
	GetInterfaceUUID() string
	// PR programs specified bitstream to port
	PR(bitstream.File, bool) error
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
