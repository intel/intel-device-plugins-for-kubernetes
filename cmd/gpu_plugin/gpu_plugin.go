// Copyright 2017-2021 Intel Corporation. All Rights Reserved.
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
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

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
	sharedDevNum       int
	enableMonitoring   bool
	resourceManagement bool
}

type devicePlugin struct {
	sysfsDir string
	devfsDir string

	options cliOptions

	gpuDeviceReg     *regexp.Regexp
	controlDeviceReg *regexp.Regexp

	scanTicker *time.Ticker
	scanDone   chan bool

	resMan *resourceManager
}

func newDevicePlugin(sysfsDir, devfsDir string, options cliOptions) *devicePlugin {
	return &devicePlugin{
		sysfsDir:         sysfsDir,
		devfsDir:         devfsDir,
		options:          options,
		gpuDeviceReg:     regexp.MustCompile(gpuDeviceRE),
		controlDeviceReg: regexp.MustCompile(controlDeviceRE),
		scanTicker:       time.NewTicker(scanPeriod),
		scanDone:         make(chan bool, 1), // buffered as we may send to it before Scan starts receiving from it
		resMan:           newResourceManager(options.resourceManagement),
	}
}

func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()
	var previouslyFound int = -1

	for {
		devTree, err := dp.scan()
		if err != nil {
			klog.Warning("Failed to scan: ", err)
		}

		found := len(devTree)
		if found != previouslyFound {
			klog.V(1).Info("GPU scan update: devices found: ", found)
			previouslyFound = found
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
	dat, err := ioutil.ReadFile(path.Join(dp.sysfsDir, name, "device/vendor"))
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
	files, err := ioutil.ReadDir(dp.sysfsDir)
	if err != nil {
		return nil, errors.Wrap(err, "Can't read sysfs folder")
	}

	var monitor []pluginapi.DeviceSpec
	devTree := dpapi.NewDeviceTree()
	for _, f := range files {
		var nodes []pluginapi.DeviceSpec

		if !dp.isCompatibleDevice(f.Name()) {
			continue
		}

		drmFiles, err := ioutil.ReadDir(path.Join(dp.sysfsDir, f.Name(), "device/drm"))
		if err != nil {
			return nil, errors.Wrap(err, "Can't read device folder")
		}

		dat, err := ioutil.ReadFile(path.Join(dp.sysfsDir, f.Name(), "device/sriov_numvfs"))
		isPFwithVFs := (err == nil && strings.TrimSpace(string(dat)) != "0")

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
			deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, nil, nil)
			for i := 0; i < dp.options.sharedDevNum; i++ {
				devID := fmt.Sprintf("%s-%d", f.Name(), i)
				// Currently only one device type (i915) is supported.
				// TODO: check model ID to differentiate device models.
				devTree.AddDevice(deviceType, devID, deviceInfo)
			}
		}
	}
	// all Intel GPUs are under single monitoring resource
	if len(monitor) > 0 {
		deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, monitor, nil, nil)
		devTree.AddDevice(monitorType, monitorID, deviceInfo)
	}

	if dp.resMan != nil {
		dp.resMan.setDevTree(devTree)
	}

	return devTree, nil
}

func (dp *devicePlugin) PostAllocate(allocateResponse *pluginapi.AllocateResponse) error {
	if dp.resMan != nil {
		return dp.resMan.reallocateWithFractionalResources(allocateResponse)
	}
	return nil
}

func main() {
	var opts cliOptions

	flag.BoolVar(&opts.enableMonitoring, "enable-monitoring", false, "whether to enable 'i915_monitoring' (= all GPUs) resource")
	flag.BoolVar(&opts.resourceManagement, "resource-manager", false, "fractional GPU resource management")
	flag.IntVar(&opts.sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same GPU device")
	flag.Parse()

	if opts.sharedDevNum < 1 {
		klog.Warning("The number of containers sharing the same GPU must greater than zero")
		os.Exit(1)
	}

	if opts.sharedDevNum == 1 && opts.resourceManagement {
		klog.Warning("Trying to use fractional resources with shared-dev-num 1 is pointless")
		os.Exit(1)
	}

	klog.V(1).Info("GPU device plugin started")

	plugin := newDevicePlugin(sysfsDrmDirectory, devfsDriDirectory, opts)
	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
