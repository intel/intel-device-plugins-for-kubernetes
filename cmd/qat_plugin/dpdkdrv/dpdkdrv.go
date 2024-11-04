// Copyright 2017-2022 Intel Corporation. All Rights Reserved.
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

// Package dpdkdrv implements QAT device plugin for DPDK driver.
package dpdkdrv

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-ini/ini"
	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

const (
	uioDevicePath      = "/dev"
	vfioDevicePath     = "/dev/vfio"
	vfioCtrlDevicePath = vfioDevicePath + "/vfio"
	uioMountPath       = "/sys/class/uio"
	pciDeviceDirectory = "/sys/bus/pci/devices"
	pciDriverDirectory = "/sys/bus/pci/drivers"
	uioSuffix          = "uio"
	iommuGroupSuffix   = "iommu_group"
	vendorPrefix       = "8086 "
	envVarPrefix       = "QAT"

	igbUio  = "igb_uio"
	vfioPci = "vfio-pci"

	// Period of device scans.
	scanPeriod = 5 * time.Second

	// Resource name to use when device capabilities are not available.
	defaultCapabilities = "generic"
)

// QAT PCI VF Device ID -> kernel QAT VF device driver mappings.
var qatDeviceDriver = map[string]string{
	"0442": "dh895xccvf",
	"0443": "dh895xccvf",
	"18a1": "c4xxxvf",
	"19e3": "c3xxxvf",
	"4941": "4xxxvf",
	"4943": "4xxxvf",
	"4945": "4xxxvf",
	"4947": "420xxvf",
	"37c9": "c6xxvf",
	"6f55": "d15xxvf",
}

// swapBDF returns ["C1:B1:A1", "C2:B2:A2"], when the given parameter is ["A1:B1:C1", "A2:B2:C2"].
func swapBDF(devstrings []string) []string {
	result := make([]string, len(devstrings))

	for n, dev := range devstrings {
		tmp := strings.Split(dev, ":")
		result[n] = fmt.Sprintf("%v:%v:%v", tmp[2], tmp[1], tmp[0])
	}

	return result
}

type preferredAllocationPolicyFunc func(*pluginapi.ContainerPreferredAllocationRequest) []string

// nonePolicy is used when no policy is specified.
func nonePolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	deviceIds := req.AvailableDeviceIDs

	return deviceIds[:req.AllocationSize]
}

// balancedPolicy is used for allocating QAT devices in balance.
func balancedPolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	// make it "FDB" and string sort and change back to "BDF"
	deviceIds := swapBDF(req.AvailableDeviceIDs)
	sort.Strings(deviceIds)
	deviceIds = swapBDF(deviceIds)

	return deviceIds[:req.AllocationSize]
}

// packedPolicy is used for allocating QAT PF devices one by one.
func packedPolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	deviceIds := req.AvailableDeviceIDs
	sort.Strings(deviceIds)
	deviceIds = deviceIds[:req.AllocationSize]

	return deviceIds
}

// DevicePlugin represents vfio based QAT plugin.
type DevicePlugin struct {
	scanTicker *time.Ticker
	scanDone   chan bool

	// Note: If restarting the plugin with a new policy, the allocations for existing pods remain with old policy.
	policy preferredAllocationPolicyFunc

	pciDriverDir    string
	pciDeviceDir    string
	dpdkDriver      string
	kernelVfDrivers []string
	maxDevices      int
}

// NewDevicePlugin returns new instance of vfio based QAT plugin.
func NewDevicePlugin(maxDevices int, kernelVfDrivers string, dpdkDriver string, preferredAllocationPolicy string) (*DevicePlugin, error) {
	if !isValidDpdkDeviceDriver(dpdkDriver) {
		return nil, errors.Errorf("wrong DPDK device driver: %s", dpdkDriver)
	}

	kernelDrivers := strings.Split(kernelVfDrivers, ",")
	for _, driver := range kernelDrivers {
		if !isValidKernelDriver(driver) {
			return nil, errors.Errorf("wrong kernel VF driver: %s", driver)
		}
	}

	allocationPolicyFunc := getAllocationPolicy(preferredAllocationPolicy)
	if allocationPolicyFunc == nil {
		return nil, errors.Errorf("wrong allocation policy: %s", preferredAllocationPolicy)
	}

	return newDevicePlugin(pciDriverDirectory, pciDeviceDirectory, maxDevices, kernelDrivers, dpdkDriver, allocationPolicyFunc), nil
}

// getAllocationPolicy returns a func that fits the policy given as a parameter. It returns nonePolicy when the flag is not set, and it returns nil when the policy is not valid value.
func getAllocationPolicy(preferredAllocationPolicy string) preferredAllocationPolicyFunc {
	switch {
	case !isFlagSet("allocation-policy"):
		return nonePolicy
	case preferredAllocationPolicy == "packed":
		return packedPolicy
	case preferredAllocationPolicy == "balanced":
		return balancedPolicy
	default:
		return nil
	}
}

// isFlagSet returns true when the flag that has the same name as the parameter is set.
func isFlagSet(name string) bool {
	set := false

	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})

	return set
}

func newDevicePlugin(pciDriverDir, pciDeviceDir string, maxDevices int, kernelVfDrivers []string, dpdkDriver string, preferredAllocationPolicyFunc preferredAllocationPolicyFunc) *DevicePlugin {
	return &DevicePlugin{
		maxDevices:      maxDevices,
		pciDriverDir:    pciDriverDir,
		pciDeviceDir:    pciDeviceDir,
		kernelVfDrivers: kernelVfDrivers,
		dpdkDriver:      dpdkDriver,
		scanTicker:      time.NewTicker(scanPeriod),
		scanDone:        make(chan bool, 1),
		policy:          preferredAllocationPolicyFunc,
	}
}

// Scan implements Scanner interface for vfio based QAT plugin.
func (dp *DevicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	for {
		devTree, err := dp.scan()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		select {
		case <-dp.scanDone:
			return nil
		case <-dp.scanTicker.C:
		}
	}
}

// Implement the PreferredAllocator interface.
func (dp *DevicePlugin) GetPreferredAllocation(rqt *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	response := &pluginapi.PreferredAllocationResponse{}

	for _, req := range rqt.ContainerRequests {
		// Add a security check here. This should never happen unless there occurs error in kubelet device plugin manager.
		if req.AllocationSize > int32(len(req.AvailableDeviceIDs)) {
			var err = errors.Errorf("AllocationSize (%d) is greater than the number of available device IDs (%d)", req.AllocationSize, len(req.AvailableDeviceIDs))
			return nil, err
		}

		IDs := dp.policy(req)
		klog.V(3).Infof("AvailableDeviceIDs: %q", req.AvailableDeviceIDs)
		klog.V(3).Infof("AllocatedDeviceIDs: %q", IDs)

		resp := &pluginapi.ContainerPreferredAllocationResponse{
			DeviceIDs: IDs,
		}

		response.ContainerResponses = append(response.ContainerResponses, resp)
	}

	return response, nil
}

func (dp *DevicePlugin) getDpdkDevice(vfBdf string) (string, error) {
	switch dp.dpdkDriver {
	case igbUio:
		uioDirPath := filepath.Join(dp.pciDeviceDir, vfBdf, uioSuffix)

		files, err := os.ReadDir(uioDirPath)
		if err != nil {
			return "", err
		}

		if len(files) == 0 {
			return "", errors.New("No devices found")
		}

		return files[0].Name(), nil

	case vfioPci:
		vfioDirPath := filepath.Join(dp.pciDeviceDir, vfBdf, iommuGroupSuffix)
		group, err := filepath.EvalSymlinks(vfioDirPath)

		if err != nil {
			return "", errors.WithStack(err)
		}

		s := filepath.Base(group)

		// If the kernel has CONFIG_VFIO_NOIOMMU enabled and the node admin
		// has explicitly set enable_unsafe_noiommu_mode VFIO parameter,
		// VFIO taints the kernel and writes "vfio-noiommu" to the IOMMU
		// group name. If these conditions are true, the /dev/vfio/ devices
		// are prefixed with "noiommu-".
		if isVfioNoIOMMU(vfioDirPath) {
			s = fmt.Sprintf("noiommu-%s", s)
		}

		return s, nil

	default:
		return "", errors.New("Unknown DPDK driver")
	}
}

func isVfioNoIOMMU(iommuGroupPath string) bool {
	if fileData, err := os.ReadFile(filepath.Join(iommuGroupPath, "name")); err == nil {
		if strings.TrimSpace(string(fileData)) == "vfio-noiommu" {
			return true
		}
	}

	return false
}

func (dp *DevicePlugin) getDpdkDeviceSpecs(dpdkDeviceName string) []pluginapi.DeviceSpec {
	switch dp.dpdkDriver {
	case igbUio:
		//Setting up with uio
		uioDev := filepath.Join(uioDevicePath, dpdkDeviceName)

		return []pluginapi.DeviceSpec{
			{
				HostPath:      uioDev,
				ContainerPath: uioDev,
				Permissions:   "rw",
			},
		}
	case vfioPci:
		//Setting up with vfio
		vfioDev := filepath.Join(vfioDevicePath, dpdkDeviceName)

		return []pluginapi.DeviceSpec{
			{
				HostPath:      vfioDev,
				ContainerPath: vfioDev,
				Permissions:   "rw",
			},
			{
				HostPath:      vfioCtrlDevicePath,
				ContainerPath: vfioCtrlDevicePath,
				Permissions:   "rw",
			},
		}
	default:
		return nil
	}
}

func (dp *DevicePlugin) getDpdkMounts(dpdkDeviceName string) []pluginapi.Mount {
	switch dp.dpdkDriver {
	case igbUio:
		//Setting up with uio mountpoints
		uioMountPoint := filepath.Join(uioMountPath, dpdkDeviceName, "/device")

		return []pluginapi.Mount{
			{
				HostPath:      uioMountPoint,
				ContainerPath: uioMountPoint,
			},
		}
	case vfioPci:
		//No mountpoint for vfio needs to be populated
		return nil
	default:
		return nil
	}
}

func readDeviceConfiguration(pfDev string) string {
	qatState, err := os.ReadFile(filepath.Join(pfDev, "qat/state"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		klog.Warningf("failed to read device state for %s: %q", filepath.Base(pfDev), err)
		return defaultCapabilities
	}

	if err == nil && strings.TrimSpace(string(qatState)) == "up" {
		qatCfgServices, err2 := os.ReadFile(filepath.Join(pfDev, "qat/cfg_services"))
		if err2 != nil && !errors.Is(err2, os.ErrNotExist) {
			klog.Warningf("failed to read services config for %s: %q", filepath.Base(pfDev), err2)
			return defaultCapabilities
		}

		if err2 == nil && len(qatCfgServices) != 0 {
			return strings.TrimSpace(string(qatCfgServices))
		}
	}

	lOpts := ini.LoadOptions{
		IgnoreInlineComment: true,
	}

	devCfgPath := filepath.Join(filepath.Dir(filepath.Join(pfDev, "../../")), "kernel/debug",
		fmt.Sprintf("qat_%s_%s/dev_cfg", getCurrentDriver(pfDev), filepath.Base(pfDev)))

	devCfg, err := ini.LoadSources(lOpts, devCfgPath)
	if err != nil {
		klog.Warningf("failed to read dev_cfg for %s: %q", filepath.Base(pfDev), err)
		return defaultCapabilities
	}

	return devCfg.Section("GENERAL").Key("ServicesEnabled").String()
}

func getDeviceHealthiness(device string, lookup map[string]string) string {
	healthiness := pluginapi.Healthy

	pfDev, err := filepath.EvalSymlinks(filepath.Join(device, "physfn"))
	if err != nil {
		klog.Warningf("failed to get PF device ID for %s: %q", filepath.Base(device), err)
		return healthiness
	}

	// VFs share one PF, so all the VFs should return the same result.
	if _, found := lookup[pfDev]; found {
		return lookup[pfDev]
	}

	// Try to find the PF's heartbeat status. If unable to, return Healthy.
	driver := getCurrentDriver(pfDev)

	hbStatusFile := filepath.Join(filepath.Dir(filepath.Join(pfDev, "../../")), "kernel/debug",
		fmt.Sprintf("qat_%s_%s/heartbeat/status", driver, filepath.Base(pfDev)))

	// If status reads "-1", the device is considered bad:
	// https://github.com/torvalds/linux/blob/v6.6-rc5/Documentation/ABI/testing/debugfs-driver-qat
	if data, err := os.ReadFile(hbStatusFile); err == nil && strings.Split(string(data), "\n")[0] == "-1" {
		healthiness = pluginapi.Unhealthy
	}

	lookup[pfDev] = healthiness

	return healthiness
}

func getDeviceCapabilities(device string) (string, error) {
	devID, err := getDeviceID(device)
	if err != nil {
		return "", errors.Wrapf(err, "cannot determine device capabilities")
	}

	devicesWithCapabilities := map[string]struct{}{
		"4941": {}, // QAT Gen4 (4xxx) VF PCI ID
		"4943": {}, // QAT Gen4 (401xx) VF PCI ID
		"4945": {}, // QAT Gen4 (402xx) VF PCI ID
		"4947": {}, // QAT Gen4 (420xx) VF PCI ID
	}

	if _, ok := devicesWithCapabilities[devID]; !ok {
		return defaultCapabilities, nil
	}

	pfDev, err := filepath.EvalSymlinks(filepath.Join(device, "physfn"))
	if err != nil {
		klog.Warningf("failed to get PF device ID for %s: %q", filepath.Base(device), err)
		return defaultCapabilities, nil
	}

	switch readDeviceConfiguration(pfDev) {
	case "sym;asym":
		return "cy", nil
	case "asym;sym":
		return "cy", nil
	case "dc":
		return "dc", nil
	case "sym":
		return "sym", nil
	case "asym":
		return "asym", nil
	case "asym;dc":
		return "asym-dc", nil
	case "dc;asym":
		return "asym-dc", nil
	case "sym;dc":
		return "sym-dc", nil
	case "dc;sym":
		return "sym-dc", nil
	default:
		return defaultCapabilities, nil
	}
}

func getDeviceID(device string) (string, error) {
	devID, err := os.ReadFile(filepath.Join(device, "device"))
	if err != nil {
		return "", errors.Wrapf(err, "failed to read device ID")
	}

	return strings.TrimPrefix(string(bytes.TrimSpace(devID)), "0x"), nil
}

func writeToDriver(path, value string) error {
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		return errors.Wrapf(err, "write to driver failed: %s", value)
	}

	return nil
}

func isValidKernelDriver(kernelvfDriver string) bool {
	for _, driver := range qatDeviceDriver {
		if driver == kernelvfDriver {
			return true
		}
	}

	return false
}

func isValidDpdkDeviceDriver(dpdkDriver string) bool {
	switch dpdkDriver {
	case igbUio, vfioPci:
		return true
	}

	return false
}

func (dp *DevicePlugin) isValidVfDeviceID(vfDevID string) bool {
	if driver, ok := qatDeviceDriver[vfDevID]; ok {
		for _, enabledDriver := range dp.kernelVfDrivers {
			if driver == enabledDriver {
				return true
			}
		}
	}

	klog.V(3).Infof("device ID %s is not a QAT device or not enabled by kernelVfDrivers.", vfDevID)

	return false
}

// PostAllocate implements PostAllocator interface for vfio based QAT plugin.
func (dp *DevicePlugin) PostAllocate(response *pluginapi.AllocateResponse) error {
	tempMap := make(map[string]string)

	for _, cresp := range response.ContainerResponses {
		counter := 0

		for k := range cresp.Envs {
			tempMap[strings.Join([]string{envVarPrefix, strconv.Itoa(counter)}, "")] = cresp.Envs[k]
			counter++
		}

		cresp.Envs = tempMap
	}

	return nil
}

func getPciDevicesWithPattern(pattern string) (pciDevices []string) {
	pciDevices = make([]string, 0)

	devs, err := filepath.Glob(pattern)
	if err != nil {
		klog.Warningf("bad pattern: %s", pattern)
		return
	}

	for _, devBdf := range devs {
		targetDev, err := filepath.EvalSymlinks(devBdf)
		if err != nil {
			klog.Warningf("unable to evaluate symlink: %s", devBdf)
			continue
		}

		pciDevices = append(pciDevices, targetDev)
	}

	return
}

func (dp *DevicePlugin) getVfDevices() []string {
	qatPfDevices := make([]string, 0)
	qatVfDevices := make([]string, 0)

	// Get PF BDFs bound to a known QAT PF driver
	for _, vfDriver := range dp.kernelVfDrivers {
		pfDriver := strings.TrimSuffix(vfDriver, "vf")
		pattern := filepath.Join(dp.pciDriverDir, pfDriver, "????:??:??.?")
		qatPfDevices = append(qatPfDevices, getPciDevicesWithPattern(pattern)...)
	}

	// Get VF devices belonging to a valid QAT PF device
	for _, qatPfDevice := range qatPfDevices {
		pattern := filepath.Join(qatPfDevice, "virtfn*")
		qatVfDevices = append(qatVfDevices, getPciDevicesWithPattern(pattern)...)
	}

	if len(qatPfDevices) > 0 {
		if len(qatVfDevices) >= dp.maxDevices {
			return qatVfDevices[:dp.maxDevices]
		}

		return qatVfDevices
	}

	// No PF devices with a QAT driver found, running in a VM?
	pattern := filepath.Join(dp.pciDeviceDir, "????:??:??.?")
	for _, pciDev := range getPciDevicesWithPattern(pattern) {
		devID, err := getDeviceID(pciDev)
		if err != nil {
			klog.Warningf("unable to read device id for device %s: %q", filepath.Base(pciDev), err)
			continue
		}

		if dp.isValidVfDeviceID(devID) {
			qatVfDevices = append(qatVfDevices, pciDev)
		}
	}

	if len(qatVfDevices) >= dp.maxDevices {
		return qatVfDevices[:dp.maxDevices]
	}

	return qatVfDevices
}

func getCurrentDriver(device string) string {
	symlink := filepath.Join(device, "driver")

	driver, err := filepath.EvalSymlinks(symlink)
	if err != nil {
		klog.Infof("no driver bound to device %q", filepath.Base(device))
		return ""
	}

	return filepath.Base(driver)
}

func (dp *DevicePlugin) scan() (dpapi.DeviceTree, error) {
	devTree := dpapi.NewDeviceTree()
	n := 0

	pfHealthLookup := map[string]string{}

	for _, vfDevice := range dp.getVfDevices() {
		vfBdf := filepath.Base(vfDevice)

		if drv := getCurrentDriver(vfDevice); drv != dp.dpdkDriver {
			if drv != "" {
				err := writeToDriver(filepath.Join(dp.pciDriverDir, drv, "unbind"), vfBdf)
				if err != nil {
					return nil, err
				}
			}

			err := writeToDriver(filepath.Join(dp.pciDriverDir, dp.dpdkDriver, "bind"), vfBdf)
			if err != nil {
				return nil, err
			}
		}

		dpdkDeviceName, err := dp.getDpdkDevice(vfBdf)
		if err != nil {
			return nil, err
		}

		cap, err := getDeviceCapabilities(vfDevice)
		if err != nil {
			return nil, err
		}

		healthiness := getDeviceHealthiness(vfDevice, pfHealthLookup)

		klog.V(1).Infof("Device %s with %s capabilities found (%s)", vfBdf, cap, healthiness)

		n = n + 1
		envs := map[string]string{
			fmt.Sprintf("%s%d", envVarPrefix, n): vfBdf,
		}

		devinfo := dpapi.NewDeviceInfo(healthiness, dp.getDpdkDeviceSpecs(dpdkDeviceName), dp.getDpdkMounts(dpdkDeviceName), envs, nil, nil)

		devTree.AddDevice(cap, vfBdf, devinfo)
	}

	return devTree, nil
}
