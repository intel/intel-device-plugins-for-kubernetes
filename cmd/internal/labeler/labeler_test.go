// Copyright 2020-2021 Intel Corporation. All Rights Reserved.
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

package labeler

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/levelzeroservice"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/pluginutils"
)

const (
	sysfsDirectory = "/sys/"
)

type mockL0Service struct {
	memSize uint64
	fail    bool
}

func (m *mockL0Service) Run(bool) {
}
func (m *mockL0Service) GetIntelIndices() ([]uint32, error) {
	return nil, nil
}
func (m *mockL0Service) GetDeviceHealth(bdfAddress string) (levelzeroservice.DeviceHealth, error) {
	return levelzeroservice.DeviceHealth{}, nil
}
func (m *mockL0Service) GetDeviceTemperature(bdfAddress string) (levelzeroservice.DeviceTemperature, error) {
	return levelzeroservice.DeviceTemperature{}, nil
}
func (m *mockL0Service) GetDeviceMemoryAmount(bdfAddress string) (uint64, error) {
	if m.fail {
		return m.memSize, os.ErrInvalid
	}

	return m.memSize, nil
}

type testcase struct {
	capabilityFile map[string][]byte
	expectedRetval error
	expectedLabels labelMap
	name           string
	sysfsfiles     map[string][]byte
	sysfsdirs      []string
	memoryOverride uint64
	memoryReserved uint64
	pciGroupLevel  uint64
}

func getTestCases() []testcase {
	return []testcase{
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/lmem_total_bytes": []byte("8086"),
			},
			name:           "successful labeling via lmem_total_bytes",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "1000",
				"gpu.intel.com/memory.max":  "8086",
				"gpu.intel.com/cards":       "card0",
				"gpu.intel.com/gpu-numbers": "0",
				"gpu.intel.com/tiles":       "1",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":       []byte("0x8086"),
				"card0/device/sriov_numvfs": []byte("1"),
			},
			name:           "pf with vfs",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/tiles": "0",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
				"card0/gt/gt1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/lmem_total_bytes": []byte("8000"),
			},
			name:           "successful labeling via card0/lmem_total_bytes and two tiles",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "1000",
				"gpu.intel.com/memory.max":  "16000",
				"gpu.intel.com/cards":       "card0",
				"gpu.intel.com/gpu-numbers": "0",
				"gpu.intel.com/tiles":       "2",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/lmem_total_bytes": []byte("8086"),
			},
			name:           "successful labeling via lmem_total_bytes and reserved memory",
			memoryOverride: 16000000000,
			memoryReserved: 86,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "1000",
				"gpu.intel.com/memory.max":  "8000",
				"gpu.intel.com/cards":       "card0",
				"gpu.intel.com/gpu-numbers": "0",
				"gpu.intel.com/tiles":       "1",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
			name:           "successful labeling via memory override",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "1000",
				"gpu.intel.com/memory.max":  "16000000000",
				"gpu.intel.com/cards":       "card0",
				"gpu.intel.com/gpu-numbers": "0",
				"gpu.intel.com/tiles":       "1",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			name:           "when all the gpus are in the same pci-group",
			memoryOverride: 16000000000,
			capabilityFile: map[string][]byte{
				"foobar": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "2000",
				"gpu.intel.com/memory.max":  "32000000000",
				"gpu.intel.com/cards":       "card0.card1",
				"gpu.intel.com/gpu-numbers": "0.1",
				"gpu.intel.com/pci-groups":  "0.1",
				"gpu.intel.com/tiles":       "2",
			},
			pciGroupLevel: 2,
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			name:           "when all the gpus belong to different pci-groups",
			memoryOverride: 16000000000,
			capabilityFile: map[string][]byte{
				"foobar": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "2000",
				"gpu.intel.com/memory.max":  "32000000000",
				"gpu.intel.com/cards":       "card0.card1",
				"gpu.intel.com/gpu-numbers": "0.1",
				"gpu.intel.com/pci-groups":  "0_1",
				"gpu.intel.com/tiles":       "2",
			},
			pciGroupLevel: 4,
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card2/device/drm/card2",
				"card3/device/drm/card3",
				"card4/device/drm/card4",
				"card5/device/drm/card5",
				"card6/device/drm/card6",
				"card7/device/drm/card7",
				"card8/device/drm/card8",
				"card9/device/drm/card9",
				"card10/device/drm/card10",
				"card11/device/drm/card11",
				"card12/device/drm/card12",
				"card13/device/drm/card13",
				"card14/device/drm/card14",
				"card15/device/drm/card15",
				"card16/device/drm/card16",
				"card17/device/drm/card17",
				"card18/device/drm/card18",
				"card19/device/drm/card19",
				"card20/device/drm/card20",
				"card21/device/drm/card21",
				"card22/device/drm/card22",
				"card23/device/drm/card23",
				"card24/device/drm/card24",
				"card25/device/drm/card25",
				"card26/device/drm/card26",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":  []byte("0x8086"),
				"card1/device/vendor":  []byte("0x8086"),
				"card2/device/vendor":  []byte("0x8086"),
				"card3/device/vendor":  []byte("0x8086"),
				"card4/device/vendor":  []byte("0x8086"),
				"card5/device/vendor":  []byte("0x8086"),
				"card6/device/vendor":  []byte("0x8086"),
				"card7/device/vendor":  []byte("0x8086"),
				"card8/device/vendor":  []byte("0x8086"),
				"card9/device/vendor":  []byte("0x8086"),
				"card10/device/vendor": []byte("0x8086"),
				"card11/device/vendor": []byte("0x8086"),
				"card12/device/vendor": []byte("0x8086"),
				"card13/device/vendor": []byte("0x8086"),
				"card14/device/vendor": []byte("0x8086"),
				"card15/device/vendor": []byte("0x8086"),
				"card16/device/vendor": []byte("0x8086"),
				"card17/device/vendor": []byte("0x8086"),
				"card18/device/vendor": []byte("0x8086"),
				"card19/device/vendor": []byte("0x8086"),
				"card20/device/vendor": []byte("0x8086"),
				"card21/device/vendor": []byte("0x8086"),
				"card22/device/vendor": []byte("0x8086"),
				"card23/device/vendor": []byte("0x8086"),
				"card24/device/vendor": []byte("0x8086"),
				"card25/device/vendor": []byte("0x8086"),
				"card26/device/vendor": []byte("0x8086"),
			},
			name:           "when there are way too many gpus, cards label gets truncated",
			memoryOverride: 16000000000,
			capabilityFile: map[string][]byte{
				"foobar": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":   "27000",
				"gpu.intel.com/memory.max":   "432000000000",
				"gpu.intel.com/cards":        "card0.card1.card10.card11.card12.card13.card14.card15.card16.ca",
				"gpu.intel.com/gpu-numbers":  "0.1.10.11.12.13.14.15.16.17.18.19.2.20.21.22.23.24.25.26.3.4.5",
				"gpu.intel.com/gpu-numbers2": "Z.6.7.8.9",
				"gpu.intel.com/tiles":        "27",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
				"card0/gt/gt1",
				"card0/gt/gt3",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/lmem_total_bytes": []byte("8000"),
			},
			name:           "successful labeling via card0/lmem_total_bytes and three tiles",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "1000",
				"gpu.intel.com/memory.max":  "24000",
				"gpu.intel.com/cards":       "card0",
				"gpu.intel.com/gpu-numbers": "0",
				"gpu.intel.com/tiles":       "3",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
				"card0/gt/gt1",
				"card1/device/drm/card1",
				"card1/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/lmem_total_bytes": []byte("8000"),
				"card1/device/vendor":    []byte("0x8086"),
				"card1/lmem_total_bytes": []byte("8000"),
			},
			name:           "successful labeling with two cards and total three tiles",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "2000",
				"gpu.intel.com/memory.max":  "24000",
				"gpu.intel.com/gpu-numbers": "0.1",
				"gpu.intel.com/cards":       "card0.card1",
				"gpu.intel.com/tiles":       "3",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
				"card1/device/drm/card1",
				"card1/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/device/numa_node": []byte("0"),
				"card0/lmem_total_bytes": []byte("8000"),
				"card1/device/vendor":    []byte("0x8086"),
				"card1/lmem_total_bytes": []byte("8000"),
				"card1/device/numa_node": []byte("1"),
			},
			name:           "successful labeling with two cards and numa node info",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":   "2000",
				"gpu.intel.com/memory.max":   "16000",
				"gpu.intel.com/gpu-numbers":  "0.1",
				"gpu.intel.com/cards":        "card0.card1",
				"gpu.intel.com/tiles":        "2",
				"gpu.intel.com/numa-gpu-map": "0-0_1-1",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card1/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0xfefe"),
				"card0/device/numa_node": []byte("0"),
				"card1/device/vendor":    []byte("0x8086"),
				"card1/lmem_total_bytes": []byte("8000"),
				"card1/device/numa_node": []byte("1"),
			},
			name:           "successful labeling with one 0x8086 card and numa node info",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":   "1000",
				"gpu.intel.com/memory.max":   "8000",
				"gpu.intel.com/gpu-numbers":  "1",
				"gpu.intel.com/cards":        "card1",
				"gpu.intel.com/tiles":        "1",
				"gpu.intel.com/numa-gpu-map": "1-1",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
				"card2/device/drm/card2",
				"card3/device/drm/card3",
				"card4/device/drm/card4",
				"card5/device/drm/card5",
				"card6/device/drm/card6",
				"card7/device/drm/card7",
				"card8/device/drm/card8",
				"card9/device/drm/card9",
				"card10/device/drm/card10",
				"card11/device/drm/card11",
				"card12/device/drm/card12",
				"card13/device/drm/card13",
				"card14/device/drm/card14",
				"card15/device/drm/card15",
				"card16/device/drm/card16",
				"card17/device/drm/card17",
				"card18/device/drm/card18",
				"card19/device/drm/card19",
				"card20/device/drm/card20",
				"card21/device/drm/card21",
				"card22/device/drm/card22",
				"card23/device/drm/card23",
				"card24/device/drm/card24",
				"card25/device/drm/card25",
				"card26/device/drm/card26",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":     []byte("0x8086"),
				"card0/device/numa_node":  []byte("0"),
				"card1/device/vendor":     []byte("0x8086"),
				"card1/device/numa_node":  []byte("0"),
				"card2/device/vendor":     []byte("0x8086"),
				"card2/device/numa_node":  []byte("0"),
				"card3/device/vendor":     []byte("0x8086"),
				"card3/device/numa_node":  []byte("0"),
				"card4/device/vendor":     []byte("0x8086"),
				"card4/device/numa_node":  []byte("0"),
				"card5/device/vendor":     []byte("0x8086"),
				"card5/device/numa_node":  []byte("0"),
				"card6/device/vendor":     []byte("0x8086"),
				"card6/device/numa_node":  []byte("0"),
				"card7/device/vendor":     []byte("0x8086"),
				"card7/device/numa_node":  []byte("0"),
				"card8/device/vendor":     []byte("0x8086"),
				"card8/device/numa_node":  []byte("0"),
				"card9/device/vendor":     []byte("0x8086"),
				"card9/device/numa_node":  []byte("2"),
				"card10/device/vendor":    []byte("0x8086"),
				"card10/device/numa_node": []byte("2"),
				"card11/device/vendor":    []byte("0x8086"),
				"card11/device/numa_node": []byte("2"),
				"card12/device/vendor":    []byte("0x8086"),
				"card12/device/numa_node": []byte("2"),
				"card13/device/vendor":    []byte("0x8086"),
				"card13/device/numa_node": []byte("1"),
				"card14/device/vendor":    []byte("0x8086"),
				"card14/device/numa_node": []byte("1"),
				"card15/device/vendor":    []byte("0x8086"),
				"card15/device/numa_node": []byte("1"),
				"card16/device/vendor":    []byte("0x8086"),
				"card16/device/numa_node": []byte("1"),
				"card17/device/vendor":    []byte("0x8086"),
				"card17/device/numa_node": []byte("1"),
				"card18/device/vendor":    []byte("0x8086"),
				"card18/device/numa_node": []byte("1"),
				"card19/device/vendor":    []byte("0x8086"),
				"card19/device/numa_node": []byte("1"),
				"card20/device/vendor":    []byte("0x8086"),
				"card20/device/numa_node": []byte("1"),
				"card21/device/vendor":    []byte("0x8086"),
				"card21/device/numa_node": []byte("1"),
				"card22/device/vendor":    []byte("0x8086"),
				"card22/device/numa_node": []byte("3"),
				"card23/device/vendor":    []byte("0x8086"),
				"card23/device/numa_node": []byte("3"),
				"card24/device/vendor":    []byte("0x8086"),
				"card24/device/numa_node": []byte("3"),
				"card25/device/vendor":    []byte("0x8086"),
				"card25/device/numa_node": []byte("3"),
				"card26/device/vendor":    []byte("0x8086"),
				"card26/device/numa_node": []byte("3"),
			},
			name:           "successful labeling with two cards and numa node info",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/cards":         "card0.card1.card10.card11.card12.card13.card14.card15.card16.ca",
				"gpu.intel.com/gpu-numbers":   "0.1.10.11.12.13.14.15.16.17.18.19.2.20.21.22.23.24.25.26.3.4.5",
				"gpu.intel.com/gpu-numbers2":  "Z.6.7.8.9",
				"gpu.intel.com/memory.max":    "432000000000",
				"gpu.intel.com/millicores":    "27000",
				"gpu.intel.com/numa-gpu-map":  "0-0.1.2.3.4.5.6.7.8_1-13.14.15.16.17.18.19.20.21_2-10.11.12.9_3",
				"gpu.intel.com/numa-gpu-map2": "Z-22.23.24.25.26",
				"gpu.intel.com/tiles":         "27",
			},
		},
		{
			sysfsdirs: []string{
				"card1/device/drm/card1",
				"card1/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card1/device/vendor":    []byte("0x8086"),
				"card1/lmem_total_bytes": []byte("8000"),
				"card1/device/numa_node": []byte("2147483648"), // max int32 + 1
			},
			name:           "too large numa node",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "1000",
				"gpu.intel.com/memory.max":  "8000",
				"gpu.intel.com/gpu-numbers": "1",
				"gpu.intel.com/cards":       "card1",
				"gpu.intel.com/tiles":       "1",
			},
		},
		{
			sysfsdirs: []string{
				"card1/device/drm/card1",
				"card1/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card1/device/vendor":    []byte("0x8086"),
				"card1/lmem_total_bytes": []byte("8000"),
				"card1/device/numa_node": []byte("32768"), // max int16 + 1
			},
			name:           "too large numa node",
			memoryOverride: 16000000000,
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":  "1000",
				"gpu.intel.com/memory.max":  "8000",
				"gpu.intel.com/gpu-numbers": "1",
				"gpu.intel.com/cards":       "card1",
				"gpu.intel.com/tiles":       "1",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/device/tile0/gt0",
				"card0/device/tile1/gt0",
				"card1/device/drm/card1",
				"card1/device/tile0/gt0",
				"card1/device/tile1/gt0",
				"card2/device/drm/card2",
				"card2/device/tile0/gt0",
				"card2/device/tile1/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/lmem_total_bytes": []byte("8000"),
				"card0/device/numa_node": []byte("1"),
				"card1/device/vendor":    []byte("0x8086"),
				"card1/lmem_total_bytes": []byte("8000"),
				"card1/device/numa_node": []byte("1"),
				"card2/device/vendor":    []byte("0x8086"),
				"card2/lmem_total_bytes": []byte("8000"),
				"card2/device/numa_node": []byte("1"),
			},
			name:           "successful labeling with three cards and with xe driver",
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":   "3000",
				"gpu.intel.com/memory.max":   "48000",
				"gpu.intel.com/gpu-numbers":  "0.1.2",
				"gpu.intel.com/cards":        "card0.card1.card2",
				"gpu.intel.com/tiles":        "6",
				"gpu.intel.com/numa-gpu-map": "1-0.1.2",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/device/tile0/gt0",
				"card0/device/tile0/gt1",
				"card0/device/tile1/gt2",
				"card0/device/tile1/gt3",
				"card0/device/tile1/gt4",
				"card0/device/tile1/gt5",
				"card1/device/drm/card1",
				"card1/device/tile0/gt0",
				"card1/device/tile0/gt1",
				"card1/device/tile1/gt2",
				"card1/device/tile1/gt4",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":    []byte("0x8086"),
				"card0/lmem_total_bytes": []byte("8000"),
				"card0/device/numa_node": []byte("1"),
				"card1/device/vendor":    []byte("0x8086"),
				"card1/lmem_total_bytes": []byte("8000"),
				"card1/device/numa_node": []byte("1"),
			},
			name:           "successful labeling with two cards, two tiles per card and multiple gts per tile",
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":   "2000",
				"gpu.intel.com/memory.max":   "32000",
				"gpu.intel.com/gpu-numbers":  "0.1",
				"gpu.intel.com/cards":        "card0.card1",
				"gpu.intel.com/tiles":        "4",
				"gpu.intel.com/numa-gpu-map": "1-0.1",
			},
		},
	}
}

func (tc *testcase) createFiles(t *testing.T, sysfs string) {
	for _, sysfsdir := range tc.sysfsdirs {
		if err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0750); err != nil {
			t.Fatalf("Failed to create fake sysfs directory: %+v", err)
		}
	}

	for filename, body := range tc.sysfsfiles {
		if err := os.WriteFile(path.Join(sysfs, filename), body, 0600); err != nil {
			t.Fatalf("Failed to create fake vendor file: %+v", err)
		}
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name           string
		str            string
		expectedResult []string
		maxLength      uint
	}{
		{
			name:           "single small enough input string passes through unsplit",
			str:            "1.2.3.4",
			maxLength:      10,
			expectedResult: []string{"1.2.3.4"},
		},
		{
			name:           "foo_bar with maxLength 4 gets split to foo_ and bar",
			str:            "foo_bar",
			maxLength:      4,
			expectedResult: []string{"foo_", "bar"},
		},
	}

	for _, test := range tests {
		result := pluginutils.Split(test.str, test.maxLength)
		if !reflect.DeepEqual(test.expectedResult, result) {
			t.Errorf("\n%q ended up with unexpected result %v vs expected %v", test.name, result, test.expectedResult)
		}
	}
}

func TestLabeling(t *testing.T) {
	root, err := os.MkdirTemp("", "test_new_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	testcases := getTestCases()

	for _, tc := range testcases {
		subroot, err := os.MkdirTemp(root, "tc")
		if err != nil {
			t.Fatalf("can't create temporary subroot directory: %+v", err)
		}

		t.Run(tc.name, func(t *testing.T) {
			err := os.MkdirAll(path.Join(subroot, "0"), 0750)
			if err != nil {
				t.Fatalf("couldn't create dir: %s", err.Error())
			}
			sysfs := path.Join(subroot, "pci0000:00/0000:00:1b.4", sysfsDirectory)

			tc.createFiles(t, sysfs)

			os.Setenv(memoryOverrideEnv, strconv.FormatUint(tc.memoryOverride, 10))
			os.Setenv(memoryReservedEnv, strconv.FormatUint(tc.memoryReserved, 10))
			os.Setenv(pciGroupingEnv, strconv.FormatUint(tc.pciGroupLevel, 10))

			labeler := newLabeler(sysfs)
			err = labeler.createLabels()
			if err != nil && tc.expectedRetval == nil ||
				err == nil && tc.expectedRetval != nil {
				t.Errorf("unexpected return value")
			}
			if tc.expectedRetval == nil && !reflect.DeepEqual(labeler.labels, tc.expectedLabels) {
				t.Errorf("test %v label mismatch with expectation:\n%v\n%v\n", tc.name, labeler.labels, tc.expectedLabels)
			}
		})
	}
}

func TestCreateAndRun(t *testing.T) {
	root, err := os.MkdirTemp("", "test_new_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	tc := getTestCases()[0]

	subroot, err := os.MkdirTemp(root, "tc")
	if err != nil {
		t.Fatalf("can't create temporary subroot directory: %+v", err)
	}

	t.Run("CreateAndPrintLabels", func(t *testing.T) {
		err := os.MkdirAll(path.Join(subroot, "0"), 0750)
		if err != nil {
			t.Fatalf("couldn't create dir: %s", err.Error())
		}
		sysfs := path.Join(subroot, "pci0000:00/0000:00:1b.4", sysfsDirectory)

		tc.createFiles(t, sysfs)

		CreateAndPrintLabels(sysfs)
	})

	waitForFileOp := func(directory, file string, eventType fsnotify.Op, duration time.Duration) bool {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			t.Fatal(err)
		}
		defer watcher.Close()

		if err := watcher.Add(directory); err != nil {
			t.Fatal(err)
		}

		timer := time.NewTimer(duration)

		for {
			select {
			case event := <-watcher.Events:
				if filepath.Base(event.Name) == file && event.Has(eventType) {
					return true
				}
			case <-timer.C:
				return false
			}
		}
	}

	t.Run("Run", func(t *testing.T) {
		err := os.MkdirAll(path.Join(subroot, "0"), 0750)
		if err != nil {
			t.Fatalf("couldn't create dir: %s", err.Error())
		}
		sysfs := path.Join(subroot, "pci0000:00/0000:00:1b.4", sysfsDirectory)

		tc.createFiles(t, sysfs)

		c := make(chan bool, 1)

		nfdLabelBase := "nfd-labelfile.txt"
		nfdLabelFile := filepath.Join(root, nfdLabelBase)

		go Run(sysfs, nfdLabelFile, time.Millisecond, c, nil, func() {})

		// Wait for the labeling timeout to trigger
		if !waitForFileOp(root, nfdLabelBase, fsnotify.Create, time.Second*2) {
			t.Error("Run didn't create label file")
		}

		err = syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
		if err != nil {
			t.Error("Calling Kill failed")
		}

		// Wait for the labeling timeout to trigger
		if !waitForFileOp(root, nfdLabelBase, fsnotify.Remove, time.Second*2) {
			// It's possible that removal happened before we started waiting.
			// Only fail the test if the file still exists.
			if _, err := os.Stat(nfdLabelFile); err == nil {
				t.Error("Run didn't remove label file")
			}
		}
	})
}

func TestL0ServiceUse(t *testing.T) {
	root, err := os.MkdirTemp("", "test_new_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	pciAddr := path.Join(root, "sys", ".devices", "0000:00:01.0")
	cardPath := path.Join(root, "sys", "card0")

	err = os.MkdirAll(pciAddr, 0750)
	if err != nil {
		t.Fatalf("couldn't create PCI dir: %s", err.Error())
	}

	err = os.MkdirAll(cardPath, 0750)
	if err != nil {
		t.Fatalf("couldn't create card dir: %s", err.Error())
	}

	err = os.Symlink(pciAddr, filepath.Join(cardPath, "device"))
	if err != nil {
		t.Fatalf("couldn't create symlink: %s", err.Error())
	}

	err = os.WriteFile(filepath.Join(root, "sys/card0/device/vendor"), []byte("0x8086"), 0600)
	if err != nil {
		t.Fatalf("couldn't write vendor file: %s", err.Error())
	}

	err = os.MkdirAll(filepath.Join(root, "sys/card0/device/drm"), 0600)
	if err != nil {
		t.Fatalf("couldn't create card drm dir: %s", err.Error())
	}

	t.Run("fetch memory from l0 service", func(t *testing.T) {
		labeler := newLabeler(filepath.Join(root, "sys"))
		labeler.levelzero = &mockL0Service{
			memSize: 12345678,
		}
		err = labeler.createLabels()

		if err != nil {
			t.Errorf("labeler didn't work with l0 service")
		}

		if labeler.labels["gpu.intel.com/memory.max"] != "12345678" {
			t.Errorf("labeler didn't get memory amount from l0 service: %v", labeler.labels)
		}
	})

	t.Run("memory fetch from l0 fails", func(t *testing.T) {
		labeler := newLabeler(filepath.Join(root, "sys"))
		labeler.levelzero = &mockL0Service{
			memSize: 0,
			fail:    true,
		}

		os.Setenv(memoryOverrideEnv, "87654321")
		err = labeler.createLabels()

		if err != nil {
			t.Errorf("labeler didn't work with l0 service")
		}

		if labeler.labels["gpu.intel.com/memory.max"] != "87654321" {
			t.Errorf("labeler got an invalid memory amount: %v", labeler.labels)
		}
	})

	t.Run("memory fetch with nil l0 service", func(t *testing.T) {
		labeler := newLabeler(filepath.Join(root, "sys"))
		labeler.levelzero = nil

		os.Setenv(memoryOverrideEnv, "87654321")
		err = labeler.createLabels()

		if err != nil {
			t.Errorf("labeler didn't work with l0 service")
		}

		if labeler.labels["gpu.intel.com/memory.max"] != "87654321" {
			t.Errorf("labeler got an invalid memory amount: %v", labeler.labels)
		}
	})
}
