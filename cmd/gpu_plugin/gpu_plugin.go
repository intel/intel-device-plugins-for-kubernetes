// Copyright 2017-2023 Intel Corporation. All Rights Reserved.
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

package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/levelzeroservice"
	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/xpumdservice"
	gpulevelzero "github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/levelzero"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

const (
	sysFsRoot       = "/sys"
	devFsRoot       = "/dev"
	wslDxgPath      = "/dev/dxg"
	wslLibPath      = "/usr/lib/wsl"
	gpuDeviceRE     = `^card[0-9]+$`
	controlDeviceRE = `^controlD[0-9]+$`
	pciAddressRE    = "^[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\\.[0-9a-f]{1}$"
	vendorString    = "0x8086"

	// Device plugin settings.
	namespace         = "gpu.intel.com"
	deviceTypeI915    = "i915"
	deviceTypeXe      = "xe"
	deviceTypeDxg     = "dxg"
	deviceTypeDefault = deviceTypeI915

	// telemetry resource settings.
	monitorSuffix           = "_monitoring"
	monitorID               = "all"
	monitorResourceCombined = "monitoring"

	// monitoring mode options.
	monitoringModeSingle = "single"
	monitoringModeSplit  = "split"

	bypathOptionNone   = "none"
	bypathOptionAll    = "all"
	bypathOptionSingle = "single"

	levelzeroAffinityMaskEnvVar = "ZE_AFFINITY_MASK"

	// Period of device scans.
	scanPeriod = 5 * time.Second

	// Default limit for temperatures.
	defaultTempLimit = 100
)

type cliOptions struct {
	preferredAllocationPolicy string
	allowIDs                  string
	denyIDs                   string
	bypathMount               string
	monitoringMode            string
	xpumdEndpoint             string
	sharedDevNum              int
	globalTempLimit           int
	memoryTempLimit           int
	gpuTempLimit              int
	enableMonitoring          bool
	wslScan                   bool
	healthManagement          bool
}

type argError struct {
	msg string
}

func (e argError) Error() string {
	return fmt.Sprintf("argument error: %s", e.msg)
}

func newArgError(msg string) error {
	return argError{
		msg: msg,
	}
}

func validatePCIDeviceIDs(pciIDList string) error {
	if pciIDList == "" {
		return nil
	}

	r := regexp.MustCompile(`^0x[0-9a-f]{4}$`)

	for id := range strings.SplitSeq(pciIDList, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			return newArgError("empty PCI ID")
		}

		if !r.MatchString(id) {
			return newArgError("invalid PCI ID: " + id)
		}
	}

	return nil
}

func (dp *devicePlugin) pciAddressForCard(cardPath, cardName string) (string, error) {
	linkPath, err := os.Readlink(cardPath)
	if err != nil {
		return "", err
	}

	// Fetches the PCI address for a drm card by reading the
	// symbolic link that the /sys/class/drm/cardX points to.
	// ../../devices/pci0000:00/0000:00:02.0/drm/card
	// -------------------------^^^^^^^^^^^^---------.
	pciAddress := filepath.Base(strings.TrimSuffix(linkPath, filepath.Join("drm", cardName)))

	if !dp.pciAddressReg.MatchString(pciAddress) {
		klog.Warningf("Invalid PCI address for %s: %s", cardPath, pciAddress)

		return "", os.ErrInvalid
	}

	return pciAddress, nil
}

func pciDeviceIDForCard(cardPath string) (string, error) {
	idPath := filepath.Join(cardPath, "device", "device")

	idBytes, err := os.ReadFile(idPath)
	if err != nil {
		return "", err
	}

	return strings.Split(string(idBytes), "\n")[0], nil
}

// Returns a slice of by-path Mounts for a pciAddress.
// by-path files are searched from the given bypathDir.
// In the by-path dir, any files that start with "pci-<pci addr>" will be added to mounts.
func (dp *devicePlugin) bypathMountsForPci(pciAddress, bypathDir string) []pluginapi.Mount {
	files, err := os.ReadDir(bypathDir)
	if err != nil {
		klog.Warningf("Failed to read by-path directory: %+v", err)

		return nil
	}

	linkPrefix := "pci-" + pciAddress

	var mounts []pluginapi.Mount

	for _, f := range files {
		if strings.HasPrefix(f.Name(), linkPrefix) {
			absPath := path.Join(bypathDir, f.Name())

			mounts = append(mounts, pluginapi.Mount{
				ContainerPath: absPath,
				HostPath:      absPath,
				ReadOnly:      true,
			})
		}
	}

	return mounts
}

func (dp *devicePlugin) bypathMountForAll() []pluginapi.Mount {
	return []pluginapi.Mount{
		{
			ContainerPath: dp.bypathDir,
			HostPath:      dp.bypathDir,
			ReadOnly:      true,
		},
	}
}

type devicePlugin struct {
	gpuDeviceReg     *regexp.Regexp
	controlDeviceReg *regexp.Regexp
	pciAddressReg    *regexp.Regexp

	scanTicker    *time.Ticker
	scanDone      chan bool
	scanResources chan bool

	levelzeroService levelzeroservice.LevelzeroService
	xpumdService     xpumdservice.XpumdService

	sysfsDrmDir    string
	devFsRoot      string
	devDriDir      string
	bypathDir      string
	healthStatuses map[string]string

	// Note: If restarting the plugin with a new policy, the allocations for existing pods remain with old policy.
	policy  preferredAllocationPolicyFunc
	options cliOptions

	bypathFound bool
}

func newDevicePlugin(sysfsDir, devFsDir string, options cliOptions) *devicePlugin {
	dp := &devicePlugin{
		sysfsDrmDir:      path.Join(sysfsDir, "class", "drm"),
		devFsRoot:        devFsDir,
		devDriDir:        path.Join(devFsDir, "dri"),
		bypathDir:        path.Join(devFsDir, "dri", "by-path"),
		options:          options,
		gpuDeviceReg:     regexp.MustCompile(gpuDeviceRE),
		controlDeviceReg: regexp.MustCompile(controlDeviceRE),
		pciAddressReg:    regexp.MustCompile(pciAddressRE),
		scanTicker:       time.NewTicker(scanPeriod),
		scanDone:         make(chan bool, 1), // buffered as we may send to it before Scan starts receiving from it
		bypathFound:      true,
		scanResources:    make(chan bool, 1),
		healthStatuses:   make(map[string]string),
	}

	switch options.preferredAllocationPolicy {
	case "balanced":
		dp.policy = balancedPolicy
	case "packed":
		dp.policy = packedPolicy
	default:
		dp.policy = nonePolicy
	}

	if !options.wslScan {
		if _, err := os.ReadDir(dp.bypathDir); err != nil {
			klog.Warningf("failed to read by-path dir: %+v", err)

			dp.bypathFound = false
		}
	}

	return dp
}

func logHealthStatusChange(card, newStatus string, statuses map[string]string) {
	prevState, found := statuses[card]
	if !found {
		klog.V(2).Infof("%s: new => %s", card, newStatus)

		statuses[card] = newStatus
	} else if prevState != newStatus {
		klog.V(2).Infof("%s: %s => %s", card, prevState, newStatus)

		statuses[card] = newStatus
	}
}

// bdfForCard resolves the PCI BDF address for a card sysfs path by following
// the "device" symlink.  It returns ("", false) when the link cannot be read.
func bdfForCard(cardPath string) (string, bool) {
	link, err := os.Readlink(filepath.Join(cardPath, "device"))
	if err != nil {
		klog.Warning("couldn't read device link for", cardPath)

		return "", false
	}

	return filepath.Base(link), true
}

func (dp *devicePlugin) healthStatusForCard(cardPath string) string {
	if dp.xpumdService != nil {
		return dp.healthStatusForCardXpumd(cardPath)
	} else if dp.levelzeroService != nil {
		return dp.healthStatusForCardLZ(cardPath)
	}

	return pluginapi.Healthy
}

// healthStatusForCardLZ checks device health using the Level-Zero service.
func (dp *devicePlugin) healthStatusForCardLZ(cardPath string) string {
	bdfAddr, ok := bdfForCard(cardPath)
	if !ok {
		return pluginapi.Healthy
	}

	health := pluginapi.Healthy

	// Check status changes after the function exits
	defer func() { logHealthStatusChange(cardPath, health, dp.healthStatuses) }()

	dh, err := dp.levelzeroService.GetDeviceHealth(bdfAddr)
	if err != nil {
		klog.Warningf("Device health retrieval failed: %v", err)

		return health
	}

	// Direct Health indicators
	klog.V(4).Infof("Health indicators: Memory=%t, Bus=%t, SoC=%t", dh.Memory, dh.Bus, dh.SoC)

	if !dh.Memory || !dh.Bus || !dh.SoC {
		health = pluginapi.Unhealthy

		return health
	}

	deviceTemps, err := dp.levelzeroService.GetDeviceTemperature(bdfAddr)
	// In case of any errors, return the current health status
	if err != nil {
		klog.Warningf("Device temperature retrieval failed: %v", err)

		return health
	}

	// Temperatures for different areas
	klog.V(4).Infof("Temperatures: Memory=%dC, GPU=%dC, Global=%dC",
		deviceTemps.Memory, deviceTemps.GPU, deviceTemps.Global)

	if deviceTemps.GPU > dp.options.gpuTempLimit ||
		deviceTemps.Global > dp.options.globalTempLimit ||
		deviceTemps.Memory > dp.options.memoryTempLimit {
		health = pluginapi.Unhealthy
	}

	return health
}

// healthStatusForCardXpumd checks device health using the xpumd service.
func (dp *devicePlugin) healthStatusForCardXpumd(cardPath string) string {
	bdfAddr, ok := bdfForCard(cardPath)
	if !ok {
		return pluginapi.Healthy
	}

	health := pluginapi.Healthy

	// Check status changes after the function exits
	defer func() { logHealthStatusChange(cardPath, health, dp.healthStatuses) }()

	healthy, err := dp.xpumdService.GetDeviceHealth(bdfAddr)
	if err != nil {
		klog.Warningf("xpumd device health retrieval failed: %v", err)

		return health
	}

	klog.V(4).Infof("xpumd health for %s: Healthy=%t", bdfAddr, healthy)

	if !healthy {
		health = pluginapi.Unhealthy
	}

	return health
}

// Implement the PreferredAllocator interface.
func (dp *devicePlugin) GetPreferredAllocation(rqt *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	response := &pluginapi.PreferredAllocationResponse{}

	for _, req := range rqt.ContainerRequests {
		klog.V(3).Infof("AvailableDeviceIDs: %q", req.AvailableDeviceIDs)
		klog.V(3).Infof("MustIncludeDeviceIDs: %q", req.MustIncludeDeviceIDs)
		klog.V(3).Infof("AllocationSize: %d", req.AllocationSize)

		// Add a security check here. This should never happen unless there occurs error in kubelet device plugin manager.
		if req.AllocationSize > int32(len(req.AvailableDeviceIDs)) {
			klog.V(3).Info("req.AllocationSize must be not greater than len(req.AvailableDeviceIDs).")

			var err = errors.Errorf("AllocationSize (%d) is greater than the number of available device IDs (%d)", req.AllocationSize, len(req.AvailableDeviceIDs))

			return nil, err
		}

		IDs := dp.policy(req)

		resp := &pluginapi.ContainerPreferredAllocationResponse{
			DeviceIDs: IDs,
		}

		response.ContainerResponses = append(response.ContainerResponses, resp)
	}

	return response, nil
}

func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	if dp.options.wslScan {
		return dp.wslGpuScan(notifier)
	} else {
		return dp.sysFsGpuScan(notifier)
	}
}

func (dp *devicePlugin) wslGpuScan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	klog.V(1).Infof("GPU (%s) resource share count = %d", deviceTypeDxg, dp.options.sharedDevNum)

	devSpecs := []pluginapi.DeviceSpec{
		{
			HostPath:      wslDxgPath,
			ContainerPath: wslDxgPath,
			Permissions:   "rw",
		},
	}

	mounts := []pluginapi.Mount{
		{
			ContainerPath: wslLibPath,
			HostPath:      wslLibPath,
			ReadOnly:      true,
		},
	}

	for {
		indices, err := dp.levelzeroService.GetIntelIndices()
		if err == nil {
			klog.V(4).Info("Intel Level-Zero indices: ", indices)

			devTree := dpapi.NewDeviceTree()

			for _, index := range indices {
				envs := map[string]string{
					levelzeroAffinityMaskEnvVar: strconv.Itoa(int(index)),
				}

				deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, devSpecs, mounts, envs, nil, nil)

				for i := 0; i < dp.options.sharedDevNum; i++ {
					devID := fmt.Sprintf("card%d-%d", index, i)
					devTree.AddDevice(deviceTypeDxg, devID, deviceInfo)
				}
			}

			notifier.Notify(devTree)
		} else {
			klog.Warning("Failed to get Intel indices from Level-Zero")
		}

		select {
		case <-dp.scanDone:
			return nil
		case <-dp.scanTicker.C:
		}
	}
}

func (dp *devicePlugin) sysFsGpuScan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	klog.V(1).Infof("GPU (%s/%s) resource share count = %d", deviceTypeI915, deviceTypeXe, dp.options.sharedDevNum)

	previousCount := map[string]int{
		deviceTypeI915: 0, deviceTypeXe: 0,
		deviceTypeXe + monitorSuffix:   0,
		deviceTypeI915 + monitorSuffix: 0,
		monitorResourceCombined:        0}

	for {
		devTree, err := dp.scan()
		if err != nil {
			klog.Warning("Failed to scan: ", err)
		}

		countChanged := false

		for name, prev := range previousCount {
			count := devTree.DeviceTypeCount(name)
			if count != prev {
				klog.V(1).Infof("GPU scan update: %d->%d '%s' resources found", prev, count, name)

				previousCount[name] = count

				countChanged = true
			}
		}

		notifier.Notify(devTree)

		// Trigger resource scan if it's enabled.
		if countChanged {
			dp.scanResources <- true
		}

		select {
		case <-dp.scanDone:
			return nil
		case <-dp.scanTicker.C:
		}
	}
}

func (dp *devicePlugin) isCompatibleDevice(name string) bool {
	if !dp.gpuDeviceReg.MatchString(name) {
		klog.V(4).Info("Not compatible device: ", name)
		return false
	}

	dat, err := os.ReadFile(path.Join(dp.sysfsDrmDir, name, "device/vendor"))
	if err != nil {
		klog.Warning("Skipping. Can't read vendor file: ", err)
		return false
	}

	if strings.TrimSpace(string(dat)) != vendorString {
		klog.V(4).Info("Non-Intel GPU: ", name)
		return false
	}

	return true
}

func (dp *devicePlugin) devPathForDrmFile(drmFile string) (devPath string, err error) {
	if dp.controlDeviceReg.MatchString(drmFile) {
		//Skipping possible drm control node
		err = os.ErrInvalid

		return
	}

	devPath = path.Join(dp.devDriDir, drmFile)
	if _, err = os.Stat(devPath); err != nil {
		return
	}

	return
}

func (dp *devicePlugin) filterOutInvalidCards(files []fs.DirEntry) []fs.DirEntry {
	filtered := []fs.DirEntry{}

	for _, f := range files {
		if !dp.isCompatibleDevice(f.Name()) {
			continue
		}

		_, err := os.Stat(path.Join(dp.sysfsDrmDir, f.Name(), "device/drm"))
		if err != nil {
			continue
		}

		allowlist := len(dp.options.allowIDs) > 0
		denylist := len(dp.options.denyIDs) > 0

		// Skip if the device is either not allowed or denied.
		if allowlist || denylist {
			pciID, err := pciDeviceIDForCard(path.Join(dp.sysfsDrmDir, f.Name()))
			if err != nil {
				klog.Warningf("Failed to get PCI ID for device %s: %+v", f.Name(), err)

				continue
			}

			if allowlist && !strings.Contains(dp.options.allowIDs, pciID) {
				klog.V(4).Infof("Skipping device %s (%s), not in allowlist: %s", f.Name(), pciID, dp.options.allowIDs)

				continue
			}

			if denylist && strings.Contains(dp.options.denyIDs, pciID) {
				klog.V(4).Infof("Skipping device %s (%s), in denylist: %s", f.Name(), pciID, dp.options.denyIDs)

				continue
			}
		}

		filtered = append(filtered, f)
	}

	return filtered
}

func (dp *devicePlugin) createDeviceSpecsFromDrmFiles(cardPath string) []pluginapi.DeviceSpec {
	specs := []pluginapi.DeviceSpec{}

	drmFiles, _ := os.ReadDir(path.Join(cardPath, "device/drm"))

	for _, drmFile := range drmFiles {
		devPath, devPathErr := dp.devPathForDrmFile(drmFile.Name())
		if devPathErr != nil {
			continue
		}

		klog.V(4).Infof("Adding %s to GPU %s", devPath, filepath.Base(cardPath))

		// even querying metrics requires device to be writable
		specs = append(specs, pluginapi.DeviceSpec{
			HostPath:      devPath,
			ContainerPath: devPath,
			Permissions:   "rw",
		})
	}
	return specs
}

// createMeiDeviceSpecs finds MEI devices associated with a GPU card by looking in
// the card's *.mei-* sysfs subdirectories and returns device specs for the
// corresponding /dev/meiX character devices.
// Device plugin cannot mount the whole /dev/ directory so verifying the existence of each
// /dev/meiX device is not possible.
func (dp *devicePlugin) createMeiDeviceSpecs(cardPath string) []pluginapi.DeviceSpec {
	specs := []pluginapi.DeviceSpec{}

	meiSysfsDirs, _ := filepath.Glob(path.Join(cardPath, "device/*.mei-*/mei"))

	klog.V(4).Info("Looking for MEI devices in sysfs dirs: ", meiSysfsDirs)

	for _, meiSysfsDir := range meiSysfsDirs {
		entries, err := os.ReadDir(meiSysfsDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			devPath := path.Join(dp.devFsRoot, entry.Name())

			klog.V(4).Infof("Adding MEI device %s for GPU %s", devPath, filepath.Base(cardPath))

			specs = append(specs, pluginapi.DeviceSpec{
				HostPath:      devPath,
				ContainerPath: devPath,
				Permissions:   "rw",
			})
		}
	}

	return specs
}

func (dp *devicePlugin) createMountsAndCDIDevices(cardPath, name string, devSpecs []pluginapi.DeviceSpec) ([]pluginapi.Mount, *cdispec.Spec) {
	mounts := []pluginapi.Mount{}

	if dp.bypathFound {
		switch dp.options.bypathMount {
		case bypathOptionAll:
			klog.V(4).Info("Using by-path mount option: all")
			mounts = dp.bypathMountForAll()
		case bypathOptionNone:
			klog.V(4).Info("Using by-path mount option: none")
			// no mounts
		case bypathOptionSingle:
			fallthrough
		default:
			klog.V(4).Info("Using by-path mount option: single/default")
			if pciAddr, pciErr := dp.pciAddressForCard(cardPath, name); pciErr == nil {
				mounts = dp.bypathMountsForPci(pciAddr, dp.bypathDir)
			}
		}
	}

	spec := &cdispec.Spec{
		Version: dpapi.CDIVersion,
		Kind:    dpapi.CDIVendor + "/gpu",
		Devices: make([]cdispec.Device, 1),
	}

	spec.Devices[0].Name = name

	cedits := &spec.Devices[0].ContainerEdits

	for idx := range devSpecs {
		cedits.DeviceNodes = append(cedits.DeviceNodes, &cdispec.DeviceNode{
			HostPath:    devSpecs[idx].HostPath,
			Path:        devSpecs[idx].ContainerPath,
			Permissions: devSpecs[idx].Permissions,
		})
	}

	for idx := range mounts {
		cedits.Mounts = append(cedits.Mounts, &cdispec.Mount{
			HostPath:      mounts[idx].HostPath,
			ContainerPath: mounts[idx].ContainerPath,
			Type:          "none",
			Options:       []string{"bind", "ro"},
		})
	}

	return mounts, spec
}

func (dp *devicePlugin) scan() (dpapi.DeviceTree, error) {
	files, err := os.ReadDir(dp.sysfsDrmDir)
	if err != nil {
		return nil, errors.Wrap(err, "Can't read sysfs folder")
	}

	monitor := make(map[string][]pluginapi.DeviceSpec, 0)

	devTree := dpapi.NewDeviceTree()
	devProps := newDeviceProperties()

	for _, f := range dp.filterOutInvalidCards(files) {
		name := f.Name()
		cardPath := path.Join(dp.sysfsDrmDir, name)

		devProps.fetch(cardPath)

		if devProps.isPfWithVfs {
			continue
		}

		devSpecs := dp.createDeviceSpecsFromDrmFiles(cardPath)

		if len(devSpecs) == 0 {
			continue
		}

		mounts, cdiDevices := dp.createMountsAndCDIDevices(cardPath, name, devSpecs)

		health := dp.healthStatusForCard(cardPath)

		deviceInfo := dpapi.NewDeviceInfo(health, devSpecs, mounts, nil, nil, cdiDevices)

		for i := 0; i < dp.options.sharedDevNum; i++ {
			devID := fmt.Sprintf("%s-%d", name, i)
			devTree.AddDevice(devProps.driver(), devID, deviceInfo)
		}

		if dp.options.enableMonitoring {
			mei := dp.createMeiDeviceSpecs(cardPath)
			monitorSpecs := append(devSpecs, mei...)

			if dp.options.monitoringMode == monitoringModeSingle {
				klog.V(4).Infof("For %s/%s, adding nodes: %+v", monitorResourceCombined, monitorID, monitorSpecs)

				monitor[monitorResourceCombined] = append(monitor[monitorResourceCombined], monitorSpecs...)
			} else {
				res := devProps.monitorResource()
				klog.V(4).Infof("For %s/%s, adding nodes: %+v", res, monitorID, monitorSpecs)

				monitor[res] = append(monitor[res], monitorSpecs...)
			}
		}
	}

	// all Intel GPUs are under single monitoring resource per KMD
	if len(monitor) > 0 {
		for resourceName, devices := range monitor {
			deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, devices, nil, nil, nil, nil)
			devTree.AddDevice(resourceName, monitorID, deviceInfo)
		}
	}

	return devTree, nil
}

func (dp *devicePlugin) Allocate(request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return nil, &dpapi.UseDefaultMethodError{}
}

func checkBasics(opts cliOptions) error {
	if opts.sharedDevNum < 1 {
		return newArgError("the number of containers sharing the same GPU must greater than zero")
	}

	var str = opts.preferredAllocationPolicy
	if !(str == "balanced" || str == "packed" || str == "none") {
		return newArgError("invalid value for preferredAllocationPolicy, the valid values: balanced, packed, none")
	}

	if len(opts.allowIDs) > 0 && len(opts.denyIDs) > 0 {
		return newArgError("cannot use both allow-ids and deny-ids options at the same time")
	}

	if err := validatePCIDeviceIDs(opts.allowIDs); err != nil {
		return fmt.Errorf("failed to validate allow-ids: %w", err)
	}

	if err := validatePCIDeviceIDs(opts.denyIDs); err != nil {
		return fmt.Errorf("failed to validate deny-ids: %w", err)
	}

	switch opts.monitoringMode {
	case monitoringModeSingle:
	case monitoringModeSplit:
	default:
		return newArgError(fmt.Sprintf("invalid value for monitoring-mode, valid values: %s, %s",
			monitoringModeSplit, monitoringModeSingle))
	}

	return nil
}

func checkArgs(opts cliOptions) error {
	if err := checkBasics(opts); err != nil {
		return fmt.Errorf("%w", err)
	}

	if opts.wslScan {
		if opts.enableMonitoring {
			return newArgError("monitoring is not supported within WSL.")
		}

		if opts.healthManagement || opts.xpumdEndpoint != "" {
			return newArgError("health management is not supported within WSL.")
		}
	}

	if opts.healthManagement && opts.xpumdEndpoint != "" {
		return newArgError("cannot use both Level-Zero sidecar and xpumd for health management.")
	}

	if opts.xpumdEndpoint != "" {
		if opts.globalTempLimit != defaultTempLimit ||
			opts.gpuTempLimit != defaultTempLimit ||
			opts.memoryTempLimit != defaultTempLimit {
			return newArgError("temperature limits do not work with xpumd health source")
		}
	}

	return nil
}

func main() {
	var (
		prefix string
		opts   cliOptions
	)

	flag.StringVar(&prefix, "prefix", "", "Prefix for devfs & sysfs paths")
	flag.BoolVar(&opts.enableMonitoring, "enable-monitoring", false, "whether to enable monitoring (= all GPUs) resource(s). See also --monitoring-mode")
	flag.StringVar(&opts.monitoringMode, "monitoring-mode", monitoringModeSingle, "monitoring resource mode when --enable-monitoring is set: single (combined gpu.intel.com/monitoring resource) or split (per-driver i915_monitoring/xe_monitoring resources)")
	flag.BoolVar(&opts.healthManagement, "health-management", false, "enable Level-Zero sidecar based GPU health management")
	flag.StringVar(&opts.xpumdEndpoint, "xpumd-endpoint", "", "enable xpumd based health management. Argument is unix socket path for the xpumd health service (e.g. /run/xpumd/intelxpuinfo.sock). When set, health data is retrieved from xpumd")
	flag.StringVar(&opts.bypathMount, "bypath", bypathOptionSingle, "DRI device 'by-path/' directory mounting options: single, none, all. Default: single")
	flag.BoolVar(&opts.wslScan, "wsl", false, "scan for / use WSL devices")
	flag.IntVar(&opts.sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same GPU device.")
	flag.IntVar(&opts.globalTempLimit, "temp-limit", defaultTempLimit, "Global temperature limit at which device is marked unhealthy. Use with health-managmement.")
	flag.IntVar(&opts.gpuTempLimit, "gpu-temp-limit", defaultTempLimit, "GPU temperature limit at which device is marked unhealthy. Use with health-managmement.")
	flag.IntVar(&opts.memoryTempLimit, "memory-temp-limit", defaultTempLimit, "Memory temperature limit at which device is marked unhealthy. Use with health-managmement.")
	flag.StringVar(&opts.preferredAllocationPolicy, "allocation-policy", "none", "modes of allocating GPU devices: balanced, packed and none")
	flag.StringVar(&opts.allowIDs, "allow-ids", "", "comma-separated list of device IDs to allow (e.g. 0x49c5,0x49c6)")
	flag.StringVar(&opts.denyIDs, "deny-ids", "", "comma-separated list of device IDs to deny (e.g. 0x49c5,0x49c6)")

	flag.Parse()

	klog.V(1).Infof("GPU device plugin started with %s preferred allocation policy", opts.preferredAllocationPolicy)

	plugin := newDevicePlugin(prefix+sysFsRoot, prefix+devFsRoot, opts)

	if err := checkArgs(plugin.options); err != nil {
		klog.Fatal("Argument check failed: ", err)
	}

	// Setup xpumd service if enabled
	setupXpumdService(plugin)
	// Setup Level-Zero service if enabled
	setupLevelZeroService(plugin)

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}

func setupLevelZeroService(plugin *devicePlugin) {
	if !plugin.options.healthManagement && !plugin.options.wslScan {
		return
	}

	klog.Info("levelzero service requested: ", gpulevelzero.DefaultUnixSocketPath)

	plugin.levelzeroService = levelzeroservice.NewLevelzero(gpulevelzero.DefaultUnixSocketPath)

	go plugin.levelzeroService.Run(true)
}

func setupXpumdService(plugin *devicePlugin) {
	if plugin.options.xpumdEndpoint == "" {
		return
	}

	klog.Info("xpumd health source requested: ", plugin.options.xpumdEndpoint)

	plugin.xpumdService = xpumdservice.NewXpumd(plugin.options.xpumdEndpoint)

	go plugin.xpumdService.Run(context.Background())
}
