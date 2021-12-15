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
	"math"
	"path/filepath"
	"strconv"
	"unsafe"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/bitstream"
	"github.com/pkg/errors"
)

const (
	dflFpgaFmePrefix  = "dfl-fme."
	dflFpgaPortPrefix = "dfl-port."
	dflFpgaFmeGlobPCI = "fpga_region/region*/dfl-fme.*"
)

// DflFME represent DFL FPGA FME device.
type DflFME struct {
	FME
	DevPath           string
	SysFsPath         string
	Name              string
	PCIDevice         *PCIDevice
	SocketID          string
	Dev               string
	CompatID          string
	BitstreamID       string
	BitstreamMetadata string
	PortsNum          string
}

// Close closes open device.
func (f *DflFME) Close() error {
	// if f.f != nil {
	// 	return f.f.Close()
	// }
	return nil
}

// NewDflFME Opens device.
func NewDflFME(dev string) (FME, error) {
	fme := &DflFME{DevPath: dev}
	if err := checkPCIDeviceType(fme); err != nil {
		return nil, err
	}

	if err := fme.updateProperties(); err != nil {
		return nil, err
	}

	return fme, nil
}

// DflPort represent DFL FPGA Port device.
type DflPort struct {
	Port
	PCIDevice *PCIDevice
	FME       FME
	DevPath   string
	SysFsPath string
	Name      string
	Dev       string
	AFUID     string
	ID        string
}

// Close closes open device.
func (f *DflPort) Close() error {
	if f.FME != nil {
		return f.FME.Close()
	}
	// if f.f != nil {
	// 	return f.f.Close()
	// }
	return nil
}

// NewDflPort Opens device.
func NewDflPort(dev string) (Port, error) {
	port := &DflPort{DevPath: dev}
	if err := checkPCIDeviceType(port); err != nil {
		return nil, err
	}

	if err := port.updateProperties(); err != nil {
		return nil, err
	}

	return port, nil
}

// common ioctls for FME and Port.
func commonDflGetAPIVersion(dev string) (int, error) {
	v, err := ioctlDev(dev, DFL_FPGA_GET_API_VERSION, 0)
	return int(v), err
}
func commonDflCheckExtension(dev string) (int, error) {
	v, err := ioctlDev(dev, DFL_FPGA_CHECK_EXTENSION, 0)
	return int(v), err
}

// GetAPIVersion  Report the version of the driver API.
// * Return: Driver API Version.
func (f *DflFME) GetAPIVersion() (int, error) {
	return commonDflGetAPIVersion(f.DevPath)
}

// CheckExtension Check whether an extension is supported.
// * Return: 0 if not supported, otherwise the extension is supported.
func (f *DflFME) CheckExtension() (int, error) {
	// return commonCheckExtension(f.f.Fd())
	return commonDflCheckExtension(f.DevPath)
}

// GetAPIVersion  Report the version of the driver API.
// * Return: Driver API Version.
func (f *DflPort) GetAPIVersion() (int, error) {
	return commonDflGetAPIVersion(f.DevPath)
}

// CheckExtension Check whether an extension is supported.
// * Return: 0 if not supported, otherwise the extension is supported.
func (f *DflPort) CheckExtension() (int, error) {
	return commonDflCheckExtension(f.DevPath)
}

// FME interfaces

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
	_, err := ioctlDev(f.DevPath, DFL_FPGA_FME_PORT_PR, uintptr(unsafe.Pointer(&value)))

	return err
}

// PortRelease releases the port per Port ID provided by caller.
// * Return: 0 on success, -errno on failure.
func (f *DflFME) PortRelease(port uint32) error {
	value := port
	_, err := ioctlDev(f.DevPath, DFL_FPGA_FME_PORT_RELEASE, uintptr(unsafe.Pointer(&value)))

	return err
}

// PortAssign assigns the port back per Port ID provided by caller.
// * Return: 0 on success, -errno on failure.
func (f *DflFME) PortAssign(port uint32) error {
	value := port
	_, err := ioctlDev(f.DevPath, DFL_FPGA_FME_PORT_ASSIGN, uintptr(unsafe.Pointer(&value)))

	return err
}

// GetDevPath returns path to device node.
func (f *DflFME) GetDevPath() string {
	return f.DevPath
}

// GetSysFsPath returns sysfs entry for FPGA FME or Port (e.g. can be used for custom errors/perf items).
func (f *DflFME) GetSysFsPath() string {
	if f.SysFsPath != "" {
		return f.SysFsPath
	}

	sysfs, err := FindSysFsDevice(f.DevPath)
	if err != nil {
		return ""
	}

	f.SysFsPath = sysfs

	return f.SysFsPath
}

// GetName returns simple FPGA name, derived from sysfs entry, can be used with /dev/ or /sys/bus/platform/.
func (f *DflFME) GetName() string {
	if f.Name != "" {
		return f.Name
	}

	f.Name = filepath.Base(f.GetSysFsPath())

	return f.Name
}

// GetPCIDevice returns PCIDevice for this device.
func (f *DflFME) GetPCIDevice() (*PCIDevice, error) {
	if f.PCIDevice != nil {
		return f.PCIDevice, nil
	}

	pci, err := NewPCIDevice(f.GetSysFsPath())
	if err != nil {
		return nil, err
	}

	f.PCIDevice = pci

	return f.PCIDevice, nil
}

// GetPortsNum returns amount of FPGA Ports associated to this FME.
func (f *DflFME) GetPortsNum() int {
	if f.PortsNum == "" {
		err := f.updateProperties()
		if err != nil {
			return -1
		}
	}

	n, err := strconv.ParseUint(f.PortsNum, 10, 32)
	if err != nil {
		return -1
	}

	return int(n)
}

// GetInterfaceUUID returns Interface UUID for FME.
func (f *DflFME) GetInterfaceUUID() (id string) {
	if f.CompatID == "" {
		err := f.updateProperties()
		if err != nil {
			return ""
		}
	}

	return f.CompatID
}

// GetSocketID returns physical socket number, in case NUMA enumeration fails.
func (f *DflFME) GetSocketID() (uint32, error) {
	if f.SocketID == "" {
		return math.MaxUint32, errors.Errorf("n/a")
	}

	id, err := strconv.ParseUint(f.SocketID, 10, 32)

	return uint32(id), err
}

// GetBitstreamID returns FME bitstream id.
func (f *DflFME) GetBitstreamID() string {
	return f.BitstreamID
}

// GetBitstreamMetadata returns FME bitstream metadata.
func (f *DflFME) GetBitstreamMetadata() string {
	return f.BitstreamMetadata
}

// Update properties from sysfs.
func (f *DflFME) updateProperties() error {
	pci, err := f.GetPCIDevice()
	if err != nil {
		return err
	}

	fileMap := map[string]*string{
		"bitstream_id":       &f.BitstreamID,
		"bitstream_metadata": &f.BitstreamMetadata,
		"dev":                &f.Dev,
		"ports_num":          &f.PortsNum,
		"socket_id":          &f.SocketID,
		"dfl-fme-region.*/fpga_region/region*/compat_id": &f.CompatID,
	}

	return readFilesInDirectory(fileMap, filepath.Join(pci.SysFsPath, dflFpgaFmeGlobPCI))
}

// Port interfaces

// PortReset Reset the FPGA Port and its AFU. No parameters are supported.
// Userspace can do Port reset at any time, e.g. during DMA or PR. But
// it should never cause any system level issue, only functional failure
// (e.g. DMA or PR operation failure) and be recoverable from the failure.
// * Return: 0 on success, -errno of failure.
func (f *DflPort) PortReset() error {
	_, err := ioctlDev(f.DevPath, DFL_FPGA_PORT_RESET, 0)
	return err
}

// PortGetInfo Retrieve information about the fpga port.
// Driver fills the info in provided struct dfl_fpga_port_info.
// * Return: 0 on success, -errno on failure.
func (f *DflPort) PortGetInfo() (ret PortInfo, err error) {
	var value DflFpgaPortInfo

	value.Argsz = uint32(unsafe.Sizeof(value))

	_, err = ioctlDev(f.DevPath, DFL_FPGA_PORT_GET_INFO, uintptr(unsafe.Pointer(&value)))
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
func (f *DflPort) PortGetRegionInfo(index uint32) (ret PortRegionInfo, err error) {
	var value DflFpgaPortRegionInfo

	value.Argsz = uint32(unsafe.Sizeof(value))
	value.Index = index

	_, err = ioctlDev(f.DevPath, DFL_FPGA_PORT_GET_REGION_INFO, uintptr(unsafe.Pointer(&value)))
	if err == nil {
		ret.Flags = value.Flags
		ret.Index = value.Index
		ret.Offset = value.Offset
		ret.Size = value.Size
	}

	return
}

// GetDevPath returns path to device node.
func (f *DflPort) GetDevPath() string {
	return f.DevPath
}

// GetSysFsPath returns sysfs entry for FPGA FME or Port (e.g. can be used for custom errors/perf items).
func (f *DflPort) GetSysFsPath() string {
	if f.SysFsPath != "" {
		return f.SysFsPath
	}

	sysfs, err := FindSysFsDevice(f.DevPath)
	if err != nil {
		return ""
	}

	f.SysFsPath = sysfs

	return f.SysFsPath
}

// GetName returns simple FPGA name, derived from sysfs entry, can be used with /dev/ or /sys/bus/platform/.
func (f *DflPort) GetName() string {
	if f.Name != "" {
		return f.Name
	}

	f.Name = filepath.Base(f.GetSysFsPath())

	return f.Name
}

// GetPCIDevice returns PCIDevice for this device.
func (f *DflPort) GetPCIDevice() (*PCIDevice, error) {
	if f.PCIDevice != nil {
		return f.PCIDevice, nil
	}

	pci, err := NewPCIDevice(f.GetSysFsPath())
	if err != nil {
		return nil, err
	}

	f.PCIDevice = pci

	return f.PCIDevice, nil
}

// GetFME returns FPGA FME device for this port.
func (f *DflPort) GetFME() (fme FME, err error) {
	if f.FME != nil {
		return f.FME, nil
	}

	pci, err := f.GetPCIDevice()
	if err != nil {
		return
	}

	if pci.PhysFn != nil {
		pci = pci.PhysFn
	}

	var dev string

	fileMap := map[string]*string{
		"dev": &dev,
	}

	if err = readFilesInDirectory(fileMap, filepath.Join(pci.SysFsPath, dflFpgaFmeGlobPCI)); err != nil {
		return
	}

	realDev, err := filepath.EvalSymlinks(filepath.Join("/dev/char", dev))
	if err != nil {
		return
	}

	fme, err = NewDflFME(realDev)
	if err != nil {
		return
	}

	f.FME = fme

	return fme, err
}

// GetPortID returns ID of the FPGA port within physical device.
func (f *DflPort) GetPortID() (uint32, error) {
	if f.ID == "" {
		err := f.updateProperties()
		if err != nil {
			return math.MaxUint32, err
		}
	}

	id, err := strconv.ParseUint(f.ID, 10, 32)

	return uint32(id), err
}

// GetAcceleratorTypeUUID returns AFU UUID for port.
func (f *DflPort) GetAcceleratorTypeUUID() (afuID string) {
	err := f.updateProperties()
	if err != nil || f.AFUID == "" {
		return ""
	}

	return f.AFUID
}

// GetInterfaceUUID returns Interface UUID for FME.
func (f *DflPort) GetInterfaceUUID() (id string) {
	fme, err := f.GetFME()
	if err != nil {
		return ""
	}
	defer fme.Close()

	return fme.GetInterfaceUUID()
}

// PR programs specified bitstream to port.
func (f *DflPort) PR(bs bitstream.File, dryRun bool) error {
	return genericPortPR(f, bs, dryRun)
}

// Update properties from sysfs.
func (f *DflPort) updateProperties() error {
	fileMap := map[string]*string{
		"afu_id": &f.AFUID,
		"dev":    &f.Dev,
		"id":     &f.ID,
	}

	return readFilesInDirectory(fileMap, f.GetSysFsPath())
}
