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

package main

import (
	"os"
	"path"
	"reflect"
	"strconv"
	"testing"
)

type testcase struct {
	expectedRetval error
	sysfsfiles     map[string][]byte
	capabilityFile map[string][]byte
	expectedLabels labelMap
	name           string
	sysfsdirs      []string
	memoryOverride uint64
	memoryReserved uint64
}

//nolint:funlen
func getTestCases() []testcase {
	return []testcase{
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":     []byte("0x8086"),
				"card0/gt/gt0/addr_range": []byte("8086"),
			},
			name:           "successful labeling via gt0/addr_range",
			memoryOverride: 16000000000,
			capabilityFile: map[string][]byte{
				"0/i915_capabilities": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":           "1000",
				"gpu.intel.com/memory.max":           "8086",
				"gpu.intel.com/platform_new.count":   "1",
				"gpu.intel.com/platform_new.present": "true",
				"gpu.intel.com/platform_new.tiles":   "1",
				"gpu.intel.com/platform_gen":         "9",
				"gpu.intel.com/cards":                "card0",
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
			capabilityFile: map[string][]byte{
				"0/i915_capabilities": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
				"card0/gt/gt1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":     []byte("0x8086"),
				"card0/gt/gt0/addr_range": []byte("8086"),
				"card0/gt/gt1/addr_range": []byte("2"),
			},
			name:           "successful labeling via gt0/addr_range and gt1/addr_range",
			memoryOverride: 16000000000,
			capabilityFile: map[string][]byte{
				"0/i915_capabilities": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":           "1000",
				"gpu.intel.com/memory.max":           "8088",
				"gpu.intel.com/platform_new.count":   "1",
				"gpu.intel.com/platform_new.present": "true",
				"gpu.intel.com/platform_new.tiles":   "2",
				"gpu.intel.com/platform_gen":         "9",
				"gpu.intel.com/cards":                "card0",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card0/gt/gt0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor":     []byte("0x8086"),
				"card0/gt/gt0/addr_range": []byte("8086"),
			},
			name:           "successful labeling via gt0/addr_range and reserved memory",
			memoryOverride: 16000000000,
			memoryReserved: 86,
			capabilityFile: map[string][]byte{
				"0/i915_capabilities": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":           "1000",
				"gpu.intel.com/memory.max":           "8000",
				"gpu.intel.com/platform_new.count":   "1",
				"gpu.intel.com/platform_new.present": "true",
				"gpu.intel.com/platform_new.tiles":   "1",
				"gpu.intel.com/platform_gen":         "9",
				"gpu.intel.com/cards":                "card0",
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
			capabilityFile: map[string][]byte{
				"0/i915_capabilities": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":           "1000",
				"gpu.intel.com/memory.max":           "16000000000",
				"gpu.intel.com/platform_new.count":   "1",
				"gpu.intel.com/platform_new.present": "true",
				"gpu.intel.com/platform_new.tiles":   "1",
				"gpu.intel.com/platform_gen":         "9",
				"gpu.intel.com/cards":                "card0",
			},
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
			name:           "when gen:capability info is missing",
			memoryOverride: 16000000000,
			capabilityFile: map[string][]byte{
				"0/i915_capabilities": []byte(
					"platform: new"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores":           "1000",
				"gpu.intel.com/memory.max":           "16000000000",
				"gpu.intel.com/platform_new.count":   "1",
				"gpu.intel.com/platform_new.present": "true",
				"gpu.intel.com/platform_new.tiles":   "1",
				"gpu.intel.com/cards":                "card0",
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
			name:           "when capability file is missing (foobar), related labels don't appear",
			memoryOverride: 16000000000,
			capabilityFile: map[string][]byte{
				"foobar": []byte(
					"platform: new\n" +
						"gen: 9"),
			},
			expectedRetval: nil,
			expectedLabels: labelMap{
				"gpu.intel.com/millicores": "2000",
				"gpu.intel.com/memory.max": "32000000000",
				"gpu.intel.com/cards":      "card0.card1",
			},
		},
	}
}

func (tc *testcase) createFiles(t *testing.T, sysfs, root string) {
	var err error
	for filename, body := range tc.capabilityFile {
		if err = os.WriteFile(path.Join(root, filename), body, 0600); err != nil {
			t.Fatalf("Failed to create fake capability file: %+v", err)
		}
	}
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

func TestLabeling(t *testing.T) {
	root, err := os.MkdirTemp("", "test_new_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	testcases := getTestCases()

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := os.MkdirAll(path.Join(root, "0"), 0750)
			if err != nil {
				t.Fatalf("couldn't create dir: %s", err.Error())
			}
			sysfs := path.Join(root, sysfsDirectory)

			tc.createFiles(t, sysfs, root)

			os.Setenv(memoryOverrideEnv, strconv.FormatUint(tc.memoryOverride, 10))
			os.Setenv(memoryReservedEnv, strconv.FormatUint(tc.memoryReserved, 10))

			labeler := newLabeler(sysfs, root)
			err = labeler.createLabels()
			if err != nil && tc.expectedRetval == nil ||
				err == nil && tc.expectedRetval != nil {
				t.Errorf("unexpected return value")
			}
			if tc.expectedRetval == nil && !reflect.DeepEqual(labeler.labels, tc.expectedLabels) {
				t.Errorf("test %v label mismatch with expectation:\n%v\n%v\n", tc.name, labeler.labels, tc.expectedLabels)
			}
			for filename := range tc.capabilityFile {
				os.Remove(path.Join(root, filename))
			}
			for filename := range tc.sysfsfiles {
				os.Remove(path.Join(sysfs, filename))
			}
		})
	}
}
