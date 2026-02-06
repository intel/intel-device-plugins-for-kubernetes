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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/pluginutils"
	"github.com/pkg/errors"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// Frequency of device scans.
	scanFrequency = 15 * time.Second

	// KubeVirt interface env names.
	kubeVirtDsaVfio  = "PCI_RESOURCE_DSA_INTEL_COM_VFIO"
	kubeVirtMDsaVfio = "MDEV_PCI_RESOURCE_DSA_INTEL_COM_VFIO"
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
	for _, cresp := range response.ContainerResponses {
		bdfs := []string{}
		for k, v := range cresp.Envs {
			if strings.HasPrefix(k, pluginutils.VFIO_BDF_PREFIX) {
				bdfs = append(bdfs, v)
			}
		}

		slices.Sort(bdfs)

		commaSeparatedBdfs := strings.Join(bdfs, ",")
		cresp.Envs[pluginutils.VFIO_BDF_PREFIX] = commaSeparatedBdfs
		cresp.Envs[kubeVirtDsaVfio] = commaSeparatedBdfs
		cresp.Envs[kubeVirtMDsaVfio] = ""
	}

	return nil
}

// scan collects devices by scanning sysfs and devfs entries.
func (dp *DevicePlugin) scan() (dpapi.DeviceTree, error) {
	filterFunc := func(dpath string) (bool, error) {
		devID, err := readFile(filepath.Join(dpath, "device"))
		if err != nil {
			return false, err
		}

		if _, ok := dp.devIDs[devID]; !ok {
			return false, nil
		}

		driver, err := filepath.EvalSymlinks(filepath.Join(dpath, "driver"))
		if err != nil {
			return false, errors.WithStack(err)
		}

		if filepath.Base(driver) != "vfio-pci" {
			return false, nil
		}

		return true, nil
	}

	return pluginutils.PciScan(filterFunc, dp.devDir)
}
