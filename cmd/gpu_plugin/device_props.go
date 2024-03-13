// Copyright 2024 Intel Corporation. All Rights Reserved.
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
	"slices"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/labeler"
	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/pluginutils"
	"k8s.io/klog/v2"
)

type DeviceProperties struct {
	currentDriver string
	drmDrivers    map[string]bool
	tileCounts    []uint64
	isPfWithVfs   bool
}

type invalidTileCountErr struct {
	error
}

func newDeviceProperties() *DeviceProperties {
	return &DeviceProperties{
		drmDrivers: make(map[string]bool),
	}
}

func (d *DeviceProperties) fetch(cardPath string) {
	d.isPfWithVfs = pluginutils.IsSriovPFwithVFs(cardPath)

	d.tileCounts = append(d.tileCounts, labeler.GetTileCount(cardPath))

	driverName, err := pluginutils.ReadDeviceDriver(cardPath)
	if err != nil {
		klog.Warningf("card (%s) doesn't have driver, using default: %s", cardPath, deviceTypeDefault)

		driverName = deviceTypeDefault
	}

	d.currentDriver = driverName
	d.drmDrivers[d.currentDriver] = true
}

func (d *DeviceProperties) drmDriverCount() int {
	return len(d.drmDrivers)
}

func (d *DeviceProperties) driver() string {
	return d.currentDriver
}

func (d *DeviceProperties) monitorResource() string {
	return d.currentDriver + monitorSuffix
}

func (d *DeviceProperties) maxTileCount() (uint64, error) {
	if len(d.tileCounts) == 0 {
		return 0, invalidTileCountErr{}
	}

	minCount := slices.Min(d.tileCounts)
	maxCount := slices.Max(d.tileCounts)

	if minCount != maxCount {
		klog.Warningf("Node's GPUs are heterogenous (min: %d, max: %d tiles)", minCount, maxCount)

		return 0, invalidTileCountErr{}
	}

	return maxCount, nil
}
