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
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/rm"
	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/labeler"
	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/pluginutils"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

const (
	sysfsDrmDirectory = "/sys/class/drm"
	devfsDriDirectory = "/dev/dri"
	nfdFeatureDir     = "/etc/kubernetes/node-feature-discovery/features.d"
	resourceFilename  = "intel-gpu-resources.txt"
	gpuDeviceRE       = `^card[0-9]+$`
	controlDeviceRE   = `^controlD[0-9]+$`
	pciAddressRE      = "^[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\\.[0-9a-f]{1}$"
	vendorString      = "0x8086"

	// Device plugin settings.
	namespace  = "gpu.intel.com"
	deviceType = "i915"

	// telemetry resource settings.
	monitorType = "i915_monitoring"
	monitorID   = "all"

	// Period of device scans.
	scanPeriod = 5 * time.Second

	// Labeler's max update interval, 5min.
	labelerMaxInterval = 5 * 60 * time.Second
)

type cliOptions struct {
	preferredAllocationPolicy string
	sharedDevNum              int
	enableMonitoring          bool
	resourceManagement        bool
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

// Returns a slice of by-path Mounts for a cardPath&Name.
// by-path files are searched from the given bypathDir.
// In the by-path dir, any files that start with "pci-<pci addr>" will be added to mounts.
func (dp *devicePlugin) bypathMountsForPci(cardPath, cardName, bypathDir string) []pluginapi.Mount {
	linkPath, err := os.Readlink(cardPath)
	if err != nil {
		return nil
	}

	// Fetches the pci address for a drm card by reading the
	// symbolic link that the /sys/class/drm/cardX points to.
	// ../../devices/pci0000:00/0000:00:02.0/drm/card
	// -------------------------^^^^^^^^^^^^---------.
	pciAddress := filepath.Base(strings.TrimSuffix(linkPath, filepath.Join("drm", cardName)))

	if !dp.pciAddressReg.MatchString(pciAddress) {
		klog.Warningf("Invalid pci address for %s: %s", cardPath, pciAddress)

		return nil
	}

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

	resMan rm.ResourceManager

	sysfsDir  string
	devfsDir  string
	bypathDir string

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
	}

	if options.resourceManagement {
		var err error

		dp.resMan, err = rm.NewResourceManager(monitorID, namespace+"/"+deviceType)
		if err != nil {
			klog.Errorf("Failed to create resource manager: %+v", err)
			return nil
		}
	}

	switch options.preferredAllocationPolicy {
	case "balanced":
		dp.policy = balancedPolicy
	case "packed":
		dp.policy = packedPolicy
	default:
		dp.policy = nonePolicy
	}

	if _, err := os.ReadDir(dp.bypathDir); err != nil {
		klog.Warningf("failed to read by-path dir: %+v", err)

		dp.bypathFound = false
	}

	return dp
}

// Implement the PreferredAllocator interface.
func (dp *devicePlugin) GetPreferredAllocation(rqt *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	if dp.resMan != nil {
		return dp.resMan.GetPreferredFractionalAllocation(rqt)
	}

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
	defer dp.scanTicker.Stop()

	klog.V(1).Infof("GPU '%s' resource share count = %d", deviceType, dp.options.sharedDevNum)

	previousCount := map[string]int{deviceType: 0, monitorType: 0}

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
		if dp.resMan != nil && countChanged {
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

func (dp *devicePlugin) scan() (dpapi.DeviceTree, error) {
	files, err := os.ReadDir(dp.sysfsDir)
	if err != nil {
		return nil, errors.Wrap(err, "Can't read sysfs folder")
	}

	var monitor []pluginapi.DeviceSpec

	devTree := dpapi.NewDeviceTree()
	rmDevInfos := rm.NewDeviceInfoMap()
	tileCounts := []uint64{}

	for _, f := range files {
		var nodes []pluginapi.DeviceSpec

		if !dp.isCompatibleDevice(f.Name()) {
			continue
		}

		cardPath := path.Join(dp.sysfsDir, f.Name())

		drmFiles, err := os.ReadDir(path.Join(cardPath, "device/drm"))
		if err != nil {
			return nil, errors.Wrap(err, "Can't read device folder")
		}

		isPFwithVFs := pluginutils.IsSriovPFwithVFs(path.Join(dp.sysfsDir, f.Name()))
		tileCounts = append(tileCounts, labeler.GetTileCount(dp.sysfsDir, f.Name()))

		for _, drmFile := range drmFiles {
			devSpec, devPath, devSpecErr := dp.devSpecForDrmFile(drmFile.Name())
			if devSpecErr != nil {
				continue
			}

			if !isPFwithVFs {
				klog.V(4).Infof("Adding %s to GPU %s", devPath, f.Name())

				nodes = append(nodes, devSpec)
			}

			if dp.options.enableMonitoring {
				klog.V(4).Infof("Adding %s to GPU %s/%s", devPath, monitorType, monitorID)

				monitor = append(monitor, devSpec)
			}
		}

		if len(nodes) > 0 {
			mounts := []pluginapi.Mount{}
			if dp.bypathFound {
				mounts = dp.bypathMountsForPci(cardPath, f.Name(), dp.bypathDir)
			}

			deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, mounts, nil, nil)

			for i := 0; i < dp.options.sharedDevNum; i++ {
				devID := fmt.Sprintf("%s-%d", f.Name(), i)
				// Currently only one device type (i915) is supported.
				// TODO: check model ID to differentiate device models.
				devTree.AddDevice(deviceType, devID, deviceInfo)

				rmDevInfos[devID] = rm.NewDeviceInfo(nodes, mounts, nil)
			}
		}
	}
	// all Intel GPUs are under single monitoring resource
	if len(monitor) > 0 {
		deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, monitor, nil, nil, nil)
		devTree.AddDevice(monitorType, monitorID, deviceInfo)
	}

	if dp.resMan != nil {
		dp.resMan.SetDevInfos(rmDevInfos)
		dp.resMan.SetTileCountPerCard(tileCounts)
	}

	return devTree, nil
}

func (dp *devicePlugin) Allocate(request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	if dp.resMan != nil {
		return dp.resMan.CreateFractionalResourceResponse(request)
	}

	return nil, &dpapi.UseDefaultMethodError{}
}

func main() {
	var (
		prefix string
		opts   cliOptions
	)

	flag.StringVar(&prefix, "prefix", "", "Prefix for devfs & sysfs paths")
	flag.BoolVar(&opts.enableMonitoring, "enable-monitoring", false, "whether to enable 'i915_monitoring' (= all GPUs) resource")
	flag.BoolVar(&opts.resourceManagement, "resource-manager", false, "fractional GPU resource management")
	flag.IntVar(&opts.sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same GPU device")
	flag.StringVar(&opts.preferredAllocationPolicy, "allocation-policy", "none", "modes of allocating GPU devices: balanced, packed and none")
	flag.Parse()

	if opts.sharedDevNum < 1 {
		klog.Error("The number of containers sharing the same GPU must greater than zero")
		os.Exit(1)
	}

	if opts.sharedDevNum == 1 && opts.resourceManagement {
		klog.Error("Trying to use fractional resources with shared-dev-num 1 is pointless")
		os.Exit(1)
	}

	var str = opts.preferredAllocationPolicy
	if !(str == "balanced" || str == "packed" || str == "none") {
		klog.Error("invalid value for preferredAllocationPolicy, the valid values: balanced, packed, none")
		os.Exit(1)
	}

	klog.V(1).Infof("GPU device plugin started with %s preferred allocation policy", opts.preferredAllocationPolicy)

	plugin := newDevicePlugin(prefix+sysfsDrmDirectory, prefix+devfsDriDirectory, opts)

	if plugin.options.resourceManagement {
		// Start labeler to export labels file for NFD.
		nfdFeatureFile := path.Join(nfdFeatureDir, resourceFilename)

		klog.V(2).Infof("NFD feature file location: %s", nfdFeatureFile)

		// Labeler catches OS signals and calls os.Exit() after receiving any.
		go labeler.Run(prefix+sysfsDrmDirectory, nfdFeatureFile,
			labelerMaxInterval, plugin.scanResources)
	}

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
