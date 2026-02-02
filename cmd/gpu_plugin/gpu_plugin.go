// Copyright 2017-2026 Intel Corporation. All Rights Reserved.
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
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/levelzeroservice"
	gpulevelzero "github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/levelzero"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/pluginutils"
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

const (
	sysfsDrmDirectory    = "/sys/class/drm"
	devfsDriDirectory    = "/dev/dri"
	wslDxgPath           = "/dev/dxg"
	wslLibPath           = "/usr/lib/wsl"
	vfioDevfsDirectory   = "/dev/vfio"
	sysfsPciBusDirectory = "/sys/bus/pci/devices"
	gpuDeviceRE          = `^card[0-9]+$`
	controlDeviceRE      = `^controlD[0-9]+$`
	pciAddressRE         = "^[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\\.[0-9a-f]{1}$"

	// Device plugin settings.
	namespace         = "gpu.intel.com"
	deviceTypeI915    = "i915"
	deviceTypeXe      = "xe"
	deviceTypeDxg     = "dxg"
	deviceTypeVfio    = "vfio"
	deviceTypeDefault = deviceTypeI915

	// telemetry resource settings.
	monitorSuffix = "_monitoring"
	monitorID     = "all"

	bypathOptionNone   = "none"
	bypathOptionAll    = "all"
	bypathOptionSingle = "single"

	levelzeroAffinityMaskEnvVar = "ZE_AFFINITY_MASK"

	// Period of device scans.
	scanPeriod = 5 * time.Second

	// Run modes.
	runModeDefault = "default"
	runModeWSL     = "wsl"
	runModeVfio    = "vfio"

	// KubeVirt interface env names.
	kubeVirtGpuVfio  = "PCI_RESOURCE_GPU_INTEL_COM_VFIO"
	kubeVirtMGpuVfio = "MDEV_PCI_RESOURCE_GPU_INTEL_COM_VFIO"
)

type cliOptions struct {
	preferredAllocationPolicy string
	allowIDs                  string
	denyIDs                   string
	bypathMount               string
	runMode                   string
	sharedDevNum              int
	globalTempLimit           int
	memoryTempLimit           int
	gpuTempLimit              int
	enableMonitoring          bool
	healthManagement          bool
}

type preferredAllocationPolicyFunc func(*pluginapi.ContainerPreferredAllocationRequest) []string

// nonePolicy is used for allocating GPU devices randomly, while trying
// to select as many individual GPU devices as requested.
func nonePolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	klog.V(2).Info("Select nonePolicy for GPU device allocation")

	devices := make(map[string]bool)
	selected := make(map[string]bool)
	neededCount := req.AllocationSize

	// When shared-dev-num is greater than 1, try to find as
	// many independent GPUs as possible, to satisfy the request.

	for _, deviceID := range req.AvailableDeviceIDs {
		device := strings.Split(deviceID, "-")[0]

		if _, found := devices[device]; !found {
			devices[device] = true
			selected[deviceID] = true
			neededCount--

			if neededCount == 0 {
				break
			}
		}
	}

	// If there were not enough independent GPUs, use remaining untaken deviceIDs.

	if neededCount > 0 {
		for _, deviceID := range req.AvailableDeviceIDs {
			if _, found := selected[deviceID]; !found {
				selected[deviceID] = true
				neededCount--

				if neededCount == 0 {
					break
				}
			}
		}
	}

	// Convert selected map into an array

	deviceIDs := []string{}

	for deviceID := range selected {
		deviceIDs = append(deviceIDs, deviceID)
	}

	klog.V(2).Infof("Allocate deviceIds: %q", deviceIDs)

	return deviceIDs
}

// balancedPolicy is used for allocating GPU devices in balance.
func balancedPolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	klog.V(2).Info("Select balancedPolicy for GPU device allocation")

	// Save the shared-devices list of each physical GPU.
	Card := make(map[string][]string)
	// Save the available shared-nums of each physical GPU.
	Count := make(map[string]int)

	for _, deviceID := range req.AvailableDeviceIDs {
		device := strings.Split(deviceID, "-")
		Card[device[0]] = append(Card[device[0]], deviceID)
		Count[device[0]]++
	}

	// Save the physical GPUs in order.
	Index := make([]string, 0)
	for key := range Count {
		Index = append(Index, key)
		sort.Strings(Card[key])
	}

	sort.Strings(Index)

	need := req.AllocationSize

	var deviceIds []string

	// We choose one device ID from the GPU card that has most shared gpu IDs each time.
	for {
		var (
			allocateCard string
			max          int
		)

		for _, key := range Index {
			if Count[key] > max {
				max = Count[key]
				allocateCard = key
			}
		}

		deviceIds = append(deviceIds, Card[allocateCard][0])
		need--

		if need == 0 {
			break
		}

		// Update Maps
		Card[allocateCard] = Card[allocateCard][1:]
		Count[allocateCard]--
	}

	klog.V(2).Infof("Allocate deviceIds: %q", deviceIds)

	return deviceIds
}

// packedPolicy is used for allocating GPU devices one by one.
func packedPolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	klog.V(2).Info("Select packedPolicy for GPU device allocation")

	deviceIds := req.AvailableDeviceIDs
	sort.Strings(deviceIds)
	deviceIds = deviceIds[:req.AllocationSize]

	klog.V(2).Infof("Allocate deviceIds: %q", deviceIds)

	return deviceIds
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

	sysfsDir       string
	devfsDir       string
	bypathDir      string
	healthStatuses map[string]string

	// Note: If restarting the plugin with a new policy, the allocations for existing pods remain with old policy.
	policy  preferredAllocationPolicyFunc
	options cliOptions

	bypathFound bool
}

func newDevicePlugin(sysfsDir, devfsDir string, options cliOptions) *devicePlugin {
	dp := &devicePlugin{
		sysfsDir:         sysfsDir,
		devfsDir:         devfsDir,
		bypathDir:        path.Join(devfsDir, "/by-path"),
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

	// Only run by-path detection on default mode.
	if options.runMode == runModeDefault {
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

func (dp *devicePlugin) healthStatusForCard(cardPath string) string {
	if dp.levelzeroService == nil {
		return pluginapi.Healthy
	}

	link, err := os.Readlink(filepath.Join(cardPath, "device"))
	if err != nil {
		klog.Warning("couldn't read device link for", cardPath)

		return pluginapi.Healthy
	}

	health := pluginapi.Healthy

	// Check status changes after the function exits
	defer func() { logHealthStatusChange(cardPath, health, dp.healthStatuses) }()

	bdfAddr := filepath.Base(link)

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
	switch dp.options.runMode {
	case runModeWSL:
		return dp.wslGpuScan(notifier)
	case runModeVfio:
		return dp.bdfGpuScan(notifier)
	default:
		return dp.drmGpuScan(notifier)
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

func (dp *devicePlugin) drmGpuScan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	klog.V(1).Infof("GPU (%s/%s) resource share count = %d", deviceTypeI915, deviceTypeXe, dp.options.sharedDevNum)

	previousCount := map[string]int{
		deviceTypeI915: 0, deviceTypeXe: 0,
		deviceTypeXe + monitorSuffix:   0,
		deviceTypeI915 + monitorSuffix: 0}

	for {
		devTree, err := dp.scan()
		if err != nil {
			klog.Fatalf("Failed to scan: %+v", err)
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

func (dp *devicePlugin) bdfGpuScan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	klog.V(1).Infof("GPU (%s)", deviceTypeVfio)

	previousCount := map[string]int{deviceTypeVfio: 0}

	filterFunc := func(dpath string) (bool, error) {
		return pluginutils.IsCompatibleGpuVfioDevice(dpath, dp.options.allowIDs, dp.options.denyIDs), nil
	}

	for {
		devTree, err := pluginutils.PciScan(filterFunc, dp.sysfsDir)
		if err != nil {
			klog.Fatalf("Failed to scan: %+v", err)
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

func (dp *devicePlugin) isSupportedDrmDevice(name string) bool {
	if !dp.gpuDeviceReg.MatchString(name) {
		klog.V(4).Info("Not supported drm device: ", name)
		return false
	}

	_, err := os.Stat(path.Join(dp.sysfsDir, name, "device/drm"))

	return err == nil
}

func (dp *devicePlugin) devPathForDrmFile(drmFile string) (devPath string, err error) {
	if dp.controlDeviceReg.MatchString(drmFile) {
		//Skipping possible drm control node
		err = os.ErrInvalid

		return
	}

	devPath = path.Join(dp.devfsDir, drmFile)
	if _, err = os.Stat(devPath); err != nil {
		return
	}

	return
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
	files, err := os.ReadDir(dp.sysfsDir)
	if err != nil {
		return nil, errors.Wrap(err, "Can't read sysfs folder")
	}

	monitor := make(map[string][]pluginapi.DeviceSpec, 0)

	devTree := dpapi.NewDeviceTree()

	for _, f := range files {
		name := f.Name()

		if !dp.isSupportedDrmDevice(name) {
			continue
		}

		cardPath := path.Join(dp.sysfsDir, name)

		dpath := filepath.Join(cardPath, "device")
		if !pluginutils.IsCompatibleGpuDevice(dpath, dp.options.allowIDs, dp.options.denyIDs) {
			continue
		}

		devSpecs := dp.createDeviceSpecsFromDrmFiles(cardPath)
		if len(devSpecs) == 0 {
			continue
		}

		mounts, cdiDevices := dp.createMountsAndCDIDevices(cardPath, name, devSpecs)
		health := dp.healthStatusForCard(cardPath)
		deviceInfo := dpapi.NewDeviceInfo(health, devSpecs, mounts, nil, nil, cdiDevices)
		driverName := pluginutils.DeviceDriverName(dpath, deviceTypeDefault)

		for i := 0; i < dp.options.sharedDevNum; i++ {
			devID := fmt.Sprintf("%s-%d", name, i)
			devTree.AddDevice(driverName, devID, deviceInfo)
		}

		if dp.options.enableMonitoring {
			res := driverName + monitorSuffix
			klog.V(4).Infof("For %s/%s, adding nodes: %+v", res, monitorID, devSpecs)

			monitor[res] = append(monitor[res], devSpecs...)
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

// PostAllocate is used only in VFIO mode to set PCI resource envs for KubeVirt.
func (dp *devicePlugin) PostAllocate(ar *pluginapi.AllocateResponse) error {
	if dp.options.runMode != runModeVfio {
		return nil
	}

	for _, cr := range ar.ContainerResponses {
		bdfs := []string{}

		for env, val := range cr.Envs {
			if env != pluginutils.VfioBdfPrefix && strings.HasPrefix(env, pluginutils.VfioBdfPrefix) {
				bdfs = append(bdfs, val)
			}
		}

		slices.Sort(bdfs)

		commaSeparatedBdfs := strings.Join(bdfs, ",")
		cr.Envs[pluginutils.VfioBdfPrefix] = commaSeparatedBdfs
		cr.Envs[kubeVirtGpuVfio] = commaSeparatedBdfs
		cr.Envs[kubeVirtMGpuVfio] = "" // empty on purpose
	}

	return nil
}

func checkAllowDenyOptions(opts cliOptions) error {
	if len(opts.allowIDs) > 0 && len(opts.denyIDs) > 0 {
		return errors.New("cannot use both allow-ids and deny-ids options at the same time. Please use only one of them.")
	}

	if err := pluginutils.ValidatePCIDeviceIDs(opts.allowIDs); err != nil {
		return fmt.Errorf("failed to validate allow-ids: %w", err)
	}

	if err := pluginutils.ValidatePCIDeviceIDs(opts.denyIDs); err != nil {
		return fmt.Errorf("failed to validate deny-ids: %w", err)
	}

	return nil
}

func checkWSLModeOptions(opts cliOptions) error {
	if opts.enableMonitoring {
		return errors.New("monitoring is not supported within WSL. Please disable monitoring.")
	}

	if opts.healthManagement {
		return errors.New("health management is not supported within WSL. Please disable health management.")
	}

	return nil
}

func checkVfioModeOptions(opts cliOptions) error {
	if opts.enableMonitoring {
		return errors.New("monitoring is not supported within VFIO mode. Please disable monitoring.")
	}

	if opts.healthManagement {
		return errors.New("health management is not supported within VFIO mode. Please disable health management.")
	}

	if opts.sharedDevNum > 1 {
		return errors.New("VFIO mode does not support shared devices. Please set shared-dev-num to 1.")
	}

	return nil
}

func main() {
	var (
		prefix string
		opts   cliOptions
	)

	flag.StringVar(&prefix, "prefix", "", "Prefix for devfs & sysfs paths")
	flag.BoolVar(&opts.enableMonitoring, "enable-monitoring", false, "whether to enable '*_monitoring' (= all GPUs) resource")
	flag.BoolVar(&opts.healthManagement, "health-management", false, "enable GPU health management")
	flag.StringVar(&opts.bypathMount, "bypath", bypathOptionSingle, "DRI device 'by-path/' directory mounting options: single, none, all. Default: single")
	flag.StringVar(&opts.runMode, "run-mode", runModeDefault, "run mode: default, wsl, vfio")
	flag.IntVar(&opts.sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same GPU device")
	flag.IntVar(&opts.globalTempLimit, "temp-limit", 100, "Global temperature limit at which device is marked unhealthy")
	flag.IntVar(&opts.gpuTempLimit, "gpu-temp-limit", 100, "GPU temperature limit at which device is marked unhealthy")
	flag.IntVar(&opts.memoryTempLimit, "memory-temp-limit", 100, "Memory temperature limit at which device is marked unhealthy")
	flag.StringVar(&opts.preferredAllocationPolicy, "allocation-policy", "none", "modes of allocating GPU devices: balanced, packed and none")
	flag.StringVar(&opts.allowIDs, "allow-ids", "", "comma-separated list of device IDs to allow (e.g. 0x49c5,0x49c6)")
	flag.StringVar(&opts.denyIDs, "deny-ids", "", "comma-separated list of device IDs to deny (e.g. 0x49c5,0x49c6)")

	flag.Parse()

	if opts.sharedDevNum < 1 {
		klog.Fatal("The number of containers sharing the same GPU must greater than zero")
	}

	var str = opts.preferredAllocationPolicy
	if !(str == "balanced" || str == "packed" || str == "none") {
		klog.Fatal("invalid value for preferredAllocationPolicy, the valid values: balanced, packed, none")
	}

	if err := checkAllowDenyOptions(opts); err != nil {
		klog.Fatal(err)
	}

	var plugin *devicePlugin

	klog.V(1).Infof("GPU device plugin started with %s preferred allocation policy", opts.preferredAllocationPolicy)

	switch opts.runMode {
	case runModeDefault:
		plugin = newDevicePlugin(prefix+sysfsDrmDirectory, prefix+devfsDriDirectory, opts)
	case runModeWSL:
		klog.Info("WSL mode requested")

		if err := checkWSLModeOptions(opts); err != nil {
			klog.Fatal(err)
		}

		plugin = newDevicePlugin(prefix+sysfsDrmDirectory, prefix+devfsDriDirectory, opts)
	case runModeVfio:
		klog.Info("VFIO mode requested")

		if err := checkVfioModeOptions(opts); err != nil {
			klog.Fatal(err)
		}

		plugin = newDevicePlugin(prefix+sysfsPciBusDirectory, prefix+vfioDevfsDirectory, opts)
	default:
		klog.Fatalf("Invalid run-mode option: %s. Supported options are: %s, %s, %s",
			opts.runMode, runModeDefault, runModeWSL, runModeVfio)
	}

	if plugin.options.healthManagement || plugin.options.runMode == runModeWSL {
		plugin.levelzeroService = levelzeroservice.NewLevelzero(gpulevelzero.DefaultUnixSocketPath)

		go plugin.levelzeroService.Run(true)
	}

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
