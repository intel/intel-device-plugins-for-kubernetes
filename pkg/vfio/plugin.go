// Copyright 2026 Intel Corporation. All Rights Reserved.
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

package vfio

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// VFIO devices directory and control device path.
	vfioPath     = "/dev/vfio"
	vfioCtrlPath = "/dev/vfio/vfio"
	// Frequency of device scans.
	scanFrequency = 15 * time.Second
	envVarPrefix  = "VFIO_BDF"
)

// DevicePlugin defines properties of the vfio device plugin.
type DevicePlugin struct {
	scanTicker *time.Ticker
	scanDone   chan bool
	devIDs     DeviceIDSet
	devDir     string
}

type DeviceIDSet map[string]struct{}

// NewDevicePlugin creates DevicePlugin.
func NewDevicePlugin(devDir string, devIDs DeviceIDSet) *DevicePlugin {
	return &DevicePlugin{
		devDir:     devDir,
		devIDs:     devIDs,
		scanTicker: time.NewTicker(scanFrequency),
		scanDone:   make(chan bool, 1),
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
	data, err := os.ReadFile(fpath)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return strings.TrimSpace(string(data)), nil
}

// PostAllocate implements PostAllocator interface for vfio device plugin. It re-maps
// VFIO_BDF<devNum counter> environment variables set by scan() to VFIO_BDF<0,1, ...>
// based on device resources requested by the container.
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

// scan collects devices by scanning sysfs and devfs entries.
func (dp *DevicePlugin) scan() (dpapi.DeviceTree, error) {
	// scan sysfs tree
	pciDevices, err := filepath.Glob(filepath.Join(dp.devDir, "????:??:??.?"))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	devTree := dpapi.NewDeviceTree()
	devNum := 0

	for _, dpath := range pciDevices {
		devID, err := readFile(filepath.Join(dpath, "device"))
		if err != nil {
			return nil, err
		}

		if _, ok := dp.devIDs[devID]; !ok {
			continue
		}

		// device belongs to an IOMMU group
		iommu_group, err := filepath.EvalSymlinks(filepath.Join(dpath, "iommu_group"))
		if err != nil {
			return nil, errors.WithStack(err)
		}

		driver, err := filepath.EvalSymlinks(filepath.Join(dpath, "driver"))
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if filepath.Base(driver) != "vfio-pci" {
			continue
		}

		devNodes := []pluginapi.DeviceSpec{
			{
				HostPath:      filepath.Join(vfioPath, filepath.Base(iommu_group)),
				ContainerPath: filepath.Join(vfioPath, filepath.Base(iommu_group)),
				Permissions:   "rw",
			},
			{
				HostPath:      vfioCtrlPath,
				ContainerPath: vfioCtrlPath,
				Permissions:   "rw",
			},
		}

		// TODO: add IOMMUFD nodes
		// iommuFdDevices, err := filepath.Glob(filepath.Join(dpath, "vfio-dev", "vfio?"))
		// if err == nil {
		// 	for _, iommuDev := range iommuFdDevices {
		// 		devNodes = append(devNodes, pluginapi.DeviceSpec{
		// 			HostPath:      filepath.Join(vfioPath, "devices", filepath.Base(iommuDev)),
		// 			ContainerPath: filepath.Join(vfioPath, "devices", filepath.Base(iommuDev)),
		// 			Permissions:   "rw",
		// 		})
		// 	}
		// }

		devNum = devNum + 1
		bdf := filepath.Base(dpath)

		envs := map[string]string{
			fmt.Sprintf("%s%d", envVarPrefix, devNum): bdf,
		}

		klog.V(4).Infof("%s (ID=%s): nodes: %+v", bdf, devID, devNodes)
		devTree.AddDevice("vfio", bdf, dpapi.NewDeviceInfo(pluginapi.Healthy, devNodes, nil, envs, nil, nil))
	}

	return devTree, nil
}
