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

package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/rm"
	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/pluginutils"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

const (
	sysfsDrmDirectory = "/sys/class/drm"
	devfsDriDirectory = "/dev/dri"
	gpuDeviceRE       = `^card[0-9]+$`
	controlDeviceRE   = `^controlD[0-9]+$`
	vendorString      = "0x8086"

	// Device plugin settings.
	namespace  = "gpu.intel.com"
	deviceType = "i915"

	// telemetry resource settings.
	monitorType = "i915_monitoring"
	monitorID   = "all"

	// Period of device scans.
	scanPeriod = 5 * time.Second
)

type cliOptions struct {
	preferredAllocationPolicy string
	sharedDevNum              int
	enableMonitoring          bool
	resourceManagement        bool
}

type preferredAllocationPolicyFunc func(*pluginapi.ContainerPreferredAllocationRequest) []string

// nonePolicy is used for allocating GPU devices randomly.
func nonePolicy(req *pluginapi.ContainerPreferredAllocationRequest) []string {
	klog.V(2).Info("Select nonePolicy for GPU device allocation")

	deviceIds := req.AvailableDeviceIDs[0:req.AllocationSize]

	klog.V(2).Infof("Allocate deviceIds: %q", deviceIds)

	return deviceIds
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

type devicePlugin struct {
	gpuDeviceReg     *regexp.Regexp
	controlDeviceReg *regexp.Regexp

	scanTicker *time.Ticker
	scanDone   chan bool

	resMan rm.ResourceManager

	sysfsDir string
	devfsDir string

	// Note: If restarting the plugin with a new policy, the allocations for existing pods remain with old policy.
	policy  preferredAllocationPolicyFunc
	options cliOptions
}

func newDevicePlugin(sysfsDir, devfsDir string, options cliOptions) *devicePlugin {
	dp := &devicePlugin{
		sysfsDir:         sysfsDir,
		devfsDir:         devfsDir,
		options:          options,
		gpuDeviceReg:     regexp.MustCompile(gpuDeviceRE),
		controlDeviceReg: regexp.MustCompile(controlDeviceRE),
		scanTicker:       time.NewTicker(scanPeriod),
		scanDone:         make(chan bool, 1), // buffered as we may send to it before Scan starts receiving from it
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

		for name, prev := range previousCount {
			count := devTree.DeviceTypeCount(name)
			if count != prev {
				klog.V(1).Infof("GPU scan update: %d->%d '%s' resources found", prev, count, name)

				previousCount[name] = count
			}
		}

		notifier.Notify(devTree)

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

func (dp *devicePlugin) scan() (dpapi.DeviceTree, error) {
	files, err := os.ReadDir(dp.sysfsDir)
	if err != nil {
		return nil, errors.Wrap(err, "Can't read sysfs folder")
	}

	var monitor []pluginapi.DeviceSpec

	devTree := dpapi.NewDeviceTree()
	rmDevInfos := rm.NewDeviceInfoMap()

	for _, f := range files {
		var nodes []pluginapi.DeviceSpec

		if !dp.isCompatibleDevice(f.Name()) {
			continue
		}

		drmFiles, err := os.ReadDir(path.Join(dp.sysfsDir, f.Name(), "device/drm"))
		if err != nil {
			return nil, errors.Wrap(err, "Can't read device folder")
		}

		isPFwithVFs := pluginutils.IsSriovPFwithVFs(path.Join(dp.sysfsDir, f.Name()))

		for _, drmFile := range drmFiles {
			if dp.controlDeviceReg.MatchString(drmFile.Name()) {
				//Skipping possible drm control node
				continue
			}

			devPath := path.Join(dp.devfsDir, drmFile.Name())
			if _, err := os.Stat(devPath); err != nil {
				continue
			}

			// even querying metrics requires device to be writable
			devSpec := pluginapi.DeviceSpec{
				HostPath:      devPath,
				ContainerPath: devPath,
				Permissions:   "rw",
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
			deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil, nil)

			for i := 0; i < dp.options.sharedDevNum; i++ {
				devID := fmt.Sprintf("%s-%d", f.Name(), i)
				// Currently only one device type (i915) is supported.
				// TODO: check model ID to differentiate device models.
				devTree.AddDevice(deviceType, devID, deviceInfo)

				rmDevInfos[devID] = rm.NewDeviceInfo(nodes, nil, nil)
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
	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
