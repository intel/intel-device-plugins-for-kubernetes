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
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
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
	cdispec "tags.cncf.io/container-device-interface/specs-go"
)

const (
	sysfsDrmDirectory = "/sys/class/drm"
	devfsDriDirectory = "/dev/dri"
	wslDxgPath        = "/dev/dxg"
	wslLibPath        = "/usr/lib/wsl"
	nfdFeatureDir     = "/etc/kubernetes/node-feature-discovery/features.d"
	resourceFilename  = "intel-gpu-resources.txt"
	gpuDeviceRE       = `^card[0-9]+$`
	controlDeviceRE   = `^controlD[0-9]+$`
	pciAddressRE      = "^[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\\.[0-9a-f]{1}$"
	vendorString      = "0x8086"

	// Device plugin settings.
	namespace         = "gpu.intel.com"
	deviceTypeI915    = "i915"
	deviceTypeXe      = "xe"
	deviceTypeDxg     = "dxg"
	deviceTypeDefault = deviceTypeI915

	// telemetry resource settings.
	monitorSuffix = "_monitoring"
	monitorID     = "all"

	levelzeroAffinityMaskEnvVar = "ZE_AFFINITY_MASK"

	// Period of device scans.
	scanPeriod = 5 * time.Second
)

type cliOptions struct {
	preferredAllocationPolicy string
	allowIDs                  string
	denyIDs                   string
	sharedDevNum              int
	temperatureLimit          int
	enableMonitoring          bool
	wslScan                   bool
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

func validatePCIDeviceIDs(pciIDList string) error {
	if pciIDList == "" {
		return nil
	}

	r := regexp.MustCompile(`^0x[0-9a-f]{4}$`)

	for id := range strings.SplitSeq(pciIDList, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			return os.ErrNotExist
		}

		if !r.MatchString(id) {
			return os.ErrInvalid
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

	dt, err := dp.levelzeroService.GetDeviceTemperature(bdfAddr)
	// In case of any errors, return the current health status
	if err != nil {
		klog.Warningf("Device temperature retrieval failed: %v", err)

		return health
	}

	limit := float64(dp.options.temperatureLimit)

	// Temperatures for different areas
	klog.V(4).Infof("Temperatures: Memory=%.1fC, GPU=%.1fC, Global=%.1fC", dh.MemoryTemperature, dh.GPUTemperature, dh.GlobalTemperature)

	if dt.GPU > limit || dt.Global > limit || dt.Memory > limit {
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
		deviceTypeI915 + monitorSuffix: 0}

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

	dat, err := os.ReadFile(path.Join(dp.sysfsDir, name, "device/vendor"))
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

func (dp *devicePlugin) devSpecForDrmFile(drmFile string) (devSpec pluginapi.DeviceSpec, devPath string, err error) {
	if dp.controlDeviceReg.MatchString(drmFile) {
		//Skipping possible drm control node
		err = os.ErrInvalid

		return
	}

	devPath = path.Join(dp.devfsDir, drmFile)
	if _, err = os.Stat(devPath); err != nil {
		return
	}

	// even querying metrics requires device to be writable
	devSpec = pluginapi.DeviceSpec{
		HostPath:      devPath,
		ContainerPath: devPath,
		Permissions:   "rw",
	}

	return
}

func (dp *devicePlugin) filterOutInvalidCards(files []fs.DirEntry) []fs.DirEntry {
	filtered := []fs.DirEntry{}

	for _, f := range files {
		if !dp.isCompatibleDevice(f.Name()) {
			continue
		}

		_, err := os.Stat(path.Join(dp.sysfsDir, f.Name(), "device/drm"))
		if err != nil {
			continue
		}

		allowlist := len(dp.options.allowIDs) > 0
		denylist := len(dp.options.denyIDs) > 0

		// Skip if the device is either not allowed or denied.
		if allowlist || denylist {
			pciID, err := pciDeviceIDForCard(path.Join(dp.sysfsDir, f.Name()))
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
		devSpec, devPath, devSpecErr := dp.devSpecForDrmFile(drmFile.Name())
		if devSpecErr != nil {
			continue
		}

		klog.V(4).Infof("Adding %s to GPU %s", devPath, filepath.Base(cardPath))

		specs = append(specs, devSpec)
	}

	return specs
}

func (dp *devicePlugin) createMountsAndCDIDevices(cardPath, name string, devSpecs []pluginapi.DeviceSpec) ([]pluginapi.Mount, *cdispec.Spec) {
	mounts := []pluginapi.Mount{}

	if dp.bypathFound {
		if pciAddr, pciErr := dp.pciAddressForCard(cardPath, name); pciErr == nil {
			mounts = dp.bypathMountsForPci(pciAddr, dp.bypathDir)
		}
	}

	spec := &cdispec.Spec{
		Version: dpapi.CDIVersion,
		Kind:    dpapi.CDIVendor + "/gpu",
		Devices: make([]cdispec.Device, 1),
	}

	spec.Devices[0].Name = name

	cedits := &spec.Devices[0].ContainerEdits

	for _, dspec := range devSpecs {
		cedits.DeviceNodes = append(cedits.DeviceNodes, &cdispec.DeviceNode{
			HostPath:    dspec.HostPath,
			Path:        dspec.ContainerPath,
			Permissions: dspec.Permissions,
		})
	}

	for _, mount := range mounts {
		cedits.Mounts = append(cedits.Mounts, &cdispec.Mount{
			HostPath:      mount.HostPath,
			ContainerPath: mount.ContainerPath,
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
	devProps := newDeviceProperties()

	for _, f := range dp.filterOutInvalidCards(files) {
		name := f.Name()
		cardPath := path.Join(dp.sysfsDir, name)

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
			res := devProps.monitorResource()
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

func checkAllowDenyOptions(opts cliOptions) bool {
	if len(opts.allowIDs) > 0 && len(opts.denyIDs) > 0 {
		klog.Error("Cannot use both allow-ids and deny-ids options at the same time. Please use only one of them.")
		return false
	}

	if err := validatePCIDeviceIDs(opts.allowIDs); err != nil {
		klog.Error("Failed to validate allow-ids: ", err)
		return false
	}

	if err := validatePCIDeviceIDs(opts.denyIDs); err != nil {
		klog.Error("Failed to validate deny-ids: ", err)
		return false
	}

	return true
}

func main() {
	var (
		prefix string
		opts   cliOptions
	)

	flag.StringVar(&prefix, "prefix", "", "Prefix for devfs & sysfs paths")
	flag.BoolVar(&opts.enableMonitoring, "enable-monitoring", false, "whether to enable '*_monitoring' (= all GPUs) resource")
	flag.BoolVar(&opts.healthManagement, "health-management", false, "enable GPU health management")
	flag.BoolVar(&opts.wslScan, "wsl", false, "scan for / use WSL devices")
	flag.IntVar(&opts.sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same GPU device")
	flag.IntVar(&opts.temperatureLimit, "temp-limit", 100, "temperature limit at which device is marked unhealthy")
	flag.StringVar(&opts.preferredAllocationPolicy, "allocation-policy", "none", "modes of allocating GPU devices: balanced, packed and none")
	flag.StringVar(&opts.allowIDs, "allow-ids", "", "comma-separated list of device IDs to allow (e.g. 0x49c5,0x49c6)")
	flag.StringVar(&opts.denyIDs, "deny-ids", "", "comma-separated list of device IDs to deny (e.g. 0x49c5,0x49c6)")

	flag.Parse()

	if opts.sharedDevNum < 1 {
		klog.Error("The number of containers sharing the same GPU must greater than zero")
		os.Exit(1)
	}

	var str = opts.preferredAllocationPolicy
	if !(str == "balanced" || str == "packed" || str == "none") {
		klog.Error("invalid value for preferredAllocationPolicy, the valid values: balanced, packed, none")
		os.Exit(1)
	}

	if !checkAllowDenyOptions(opts) {
		klog.Error("Invalid allow/deny options.")

		os.Exit(1)
	}

	klog.V(1).Infof("GPU device plugin started with %s preferred allocation policy", opts.preferredAllocationPolicy)

	plugin := newDevicePlugin(prefix+sysfsDrmDirectory, prefix+devfsDriDirectory, opts)

	if plugin.options.wslScan {
		klog.Info("WSL mode requested")

		if plugin.options.enableMonitoring {
			klog.Error("Monitoring is not supported within WSL. Please disable monitoring.")

			os.Exit(1)
		}

		if plugin.options.healthManagement {
			klog.Error("Health management is not supported within WSL. Please disable health management.")

			os.Exit(1)
		}
	}

	if plugin.options.healthManagement || plugin.options.wslScan {
		plugin.levelzeroService = levelzeroservice.NewLevelzero(gpulevelzero.DefaultUnixSocketPath)

		go plugin.levelzeroService.Run(true)
	}

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
