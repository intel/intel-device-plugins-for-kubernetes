// Copyright 2020 Intel Corporation. All Rights Reserved.
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

package idxd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// Character devices directory.
	charDevDir = "/dev/char"
	// Frequency of device scans.
	scanFrequency = 5 * time.Second
)

// getDevNodesFunc type allows overriding filesystem APIs (os.Stat, stat.Sys, etc) in tests.
type getDevNodesFunc func(devDir, charDevDir, wqName string) ([]pluginapi.DeviceSpec, error)

// DevicePlugin defines properties of the idxd device plugin.
type DevicePlugin struct {
	sysfsDir     string
	statePattern string
	devDir       string
	charDevDir   string
	sharedDevNum int
	scanTicker   *time.Ticker
	scanDone     chan bool
	getDevNodes  getDevNodesFunc
}

// NewDevicePlugin creates DevicePlugin.
func NewDevicePlugin(sysfsDir, statePattern, devDir string, sharedDevNum int) *DevicePlugin {
	return &DevicePlugin{
		sysfsDir:     sysfsDir,
		statePattern: statePattern,
		devDir:       devDir,
		charDevDir:   charDevDir,
		sharedDevNum: sharedDevNum,
		scanTicker:   time.NewTicker(scanFrequency),
		scanDone:     make(chan bool, 1),
		getDevNodes:  getDevNodes,
	}
}

// Scan discovers devices and reports them to the upper level API.
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

func readFile(fpath string) (string, error) {
	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return strings.TrimSpace(string(data)), nil
}

// getDevNodes collects device nodes that belong to working queue.
func getDevNodes(devDir, charDevDir, wqName string) ([]pluginapi.DeviceSpec, error) {
	// check if /dev/dsa/<work queue> device node exists
	devPath := path.Join(devDir, wqName)
	stat, err := os.Stat(devPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// Check if it's a character device
	if stat.Mode()&os.ModeCharDevice == 0 {
		return nil, errors.Errorf("%s is not a character device", devPath)
	}

	// get /dev/char/<major>:<minor> symlink for the device node
	// as libaccel-config requires it
	rdev := stat.Sys().(*syscall.Stat_t).Rdev
	charDevPath := path.Join(charDevDir, fmt.Sprintf("%d:%d", unix.Major(rdev), unix.Minor(rdev)))
	stat, err = os.Lstat(charDevPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if stat.Mode()&os.ModeSymlink == 0 {
		return nil, errors.Errorf("%s is not a symlink", charDevPath)
	}
	// Check if symlink points to the correct device node
	destPath, err := filepath.EvalSymlinks(charDevPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if destPath != devPath {
		return nil, errors.Errorf("%s points to %s instead of device node %s", charDevPath, destPath, devPath)
	}

	// report device node and /dev/char/<major>:<minor> symlink
	// as libaccel-config works with a symlink
	return []pluginapi.DeviceSpec{
		{
			HostPath:      devPath,
			ContainerPath: devPath,
			Permissions:   "rw",
		},
		{
			HostPath:      charDevPath,
			ContainerPath: charDevPath,
			Permissions:   "rw",
		}}, nil
}

// scan collects devices by scanning sysfs and devfs entries.
func (dp *DevicePlugin) scan() (dpapi.DeviceTree, error) {
	// scan sysfs tree
	matches, err := filepath.Glob(dp.statePattern)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	devTree := dpapi.NewDeviceTree()

	for _, fpath := range matches {
		// Read queue state entry
		wqState, err := readFile(fpath)
		if err != nil {
			return nil, err
		}

		if wqState != "enabled" {
			continue
		}

		// Read queue mode
		queueDir := filepath.Dir(fpath)
		wqMode, err := readFile(path.Join(queueDir, "mode"))
		if err != nil {
			return nil, err
		}

		// Read queue type
		wqType, err := readFile(path.Join(queueDir, "type"))
		if err != nil {
			return nil, err
		}

		wqName := filepath.Base(queueDir)
		devNodes := []pluginapi.DeviceSpec{}

		if wqType == "user" {
			devNodes, err = dp.getDevNodes(dp.devDir, dp.charDevDir, wqName)
			if err != nil {
				return nil, err
			}
		}

		amount := dp.sharedDevNum
		if wqMode != "shared" {
			amount = 1
		}
		klog.V(4).Infof("%s: amount: %d, type: %s, mode: %s, nodes: %+v", wqName, amount, wqType, wqMode, devNodes)
		for i := 0; i < amount; i++ {
			deviceType := fmt.Sprintf("wq-%s-%s", wqType, wqMode)
			deviceID := fmt.Sprintf("%s-%s-%d", deviceType, wqName, i)
			devTree.AddDevice(deviceType, deviceID, dpapi.NewDeviceInfo(pluginapi.Healthy, devNodes, nil, nil))
		}
	}

	return devTree, nil
}
