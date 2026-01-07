// Copyright 2025 Intel Corporation. All Rights Reserved.
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
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

const (
	sysfAccelDirectory  = "/sys/class/accel"
	devfsAccelDirectory = "/dev/accel"
	npuDeviceRE         = `^accel[0-9]+$`
	vendorString        = "0x8086"

	// Device plugin settings.
	namespace     = "npu.intel.com"
	deviceTypeNpu = "accel"

	// Period of device scans.
	scanPeriod = 5 * time.Second
)

var npuIDs = []string{
	"0x7e4c", // Core Ultra Series 1
	"0x643e", // Core Ultra 200V Series
	"0xad1d", // Core Ultra Series 2
	"0x7d1d", // Core Ultra Series 2 (H)
	"0xb03e", // Core Ultra Series 3
	"0xfd3e", // WCL https://github.com/torvalds/linux/blob/f0b9d8eb98dfee8d00419aa07543bdc2c1a44fb1/drivers/accel/ivpu/ivpu_drv.h#L29
	"0xd71d", // NVL https://github.com/torvalds/linux/blob/f0b9d8eb98dfee8d00419aa07543bdc2c1a44fb1/drivers/accel/ivpu/ivpu_drv.h#L30
}

type cliOptions struct {
	sharedDevNum int
}

type devicePlugin struct {
	npuDeviceReg *regexp.Regexp

	scanTicker *time.Ticker
	scanDone   chan bool

	sysfsDir string
	devfsDir string

	options cliOptions
}

func newDevicePlugin(sysfsDir, devfsDir string, options cliOptions) *devicePlugin {
	dp := &devicePlugin{
		sysfsDir:     sysfsDir,
		devfsDir:     devfsDir,
		options:      options,
		npuDeviceReg: regexp.MustCompile(npuDeviceRE),
		scanTicker:   time.NewTicker(scanPeriod),
		scanDone:     make(chan bool, 1), // buffered as we may send to it before Scan starts receiving from it
	}

	return dp
}

func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()

	klog.V(1).Infof("NPU (%s) resource share count = %d", deviceTypeNpu, dp.options.sharedDevNum)

	previousCount := 0
	devType := fmt.Sprintf("%s/%s", namespace, deviceTypeNpu)

	for {
		devTree, err := dp.scan()
		if err != nil {
			klog.Errorf("NPU scan failed: %v", err)
			return errors.Wrap(err, "NPU scan failed")
		}

		count := devTree.DeviceTypeCount(devType)
		if count != previousCount {
			klog.V(1).Infof("NPU scan update: %d->%d '%s' resources found", previousCount, count, devType)

			previousCount = count
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
	if !dp.npuDeviceReg.MatchString(name) {
		klog.V(4).Info("Incompatible device: ", name)
		return false
	}

	dat, err := os.ReadFile(path.Join(dp.sysfsDir, name, "device/vendor"))
	if err != nil {
		klog.Warning("Skipping. Can't read vendor file: ", err)
		return false
	}

	if strings.TrimSpace(string(dat)) != vendorString {
		klog.V(4).Info("Non-Intel accelerator device: ", name)
		return false
	}

	dat, err = os.ReadFile(path.Join(dp.sysfsDir, name, "device/device"))
	if err != nil {
		klog.Warning("Skipping. Can't read device file: ", err)
		return false
	}

	datStr := strings.Split(string(dat), "\n")[0]
	if !slices.Contains(npuIDs, datStr) {
		klog.Warning("Unknown device ID: ", datStr)
		return false
	}

	return true
}

func (dp *devicePlugin) scan() (dpapi.DeviceTree, error) {
	files, err := os.ReadDir(dp.sysfsDir)
	if err != nil {
		return nil, errors.Wrap(err, "Can't read sysfs directory")
	}

	devTree := dpapi.NewDeviceTree()

	for _, f := range files {
		name := f.Name()

		if !dp.isCompatibleDevice(name) {
			continue
		}

		devPath := path.Join(dp.devfsDir, name)
		if _, err = os.Stat(devPath); err != nil {
			continue
		}

		// even querying metrics requires device to be writable
		devSpec := []pluginapi.DeviceSpec{{
			HostPath:      devPath,
			ContainerPath: devPath,
			Permissions:   "rw",
		},
		}

		deviceInfo := dpapi.NewDeviceInfo(pluginapi.Healthy, devSpec, nil, nil, nil, nil)

		for i := 0; i < dp.options.sharedDevNum; i++ {
			devID := fmt.Sprintf("%s-%d", name, i)
			devTree.AddDevice("accel", devID, deviceInfo)
		}
	}

	return devTree, nil
}

func (dp *devicePlugin) Allocate(request *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return nil, &dpapi.UseDefaultMethodError{}
}

func main() {
	var (
		prefix string
		opts   cliOptions
	)

	flag.StringVar(&prefix, "prefix", "", "Prefix for devfs & sysfs paths")
	flag.IntVar(&opts.sharedDevNum, "shared-dev-num", 1, "number of containers sharing the same NPU device")
	flag.Parse()

	if opts.sharedDevNum < 1 {
		klog.Error("The number of containers sharing the same NPU must greater than zero")
		os.Exit(1)
	}

	klog.V(1).Infof("NPU device plugin started")

	plugin := newDevicePlugin(prefix+sysfAccelDirectory, prefix+devfsAccelDirectory, opts)

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
