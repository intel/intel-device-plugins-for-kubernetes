// Copyright 2018 Intel Corporation. All Rights Reserved.
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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
	"github.com/pkg/errors"
)

func init() {
	debug.Activate()
}

func createTestDirs(sysfs string, sysfsDirs []string, sysfsFiles map[string][]byte) error {
	for _, sysfsdir := range sysfsDirs {
		err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}
	for filename, body := range sysfsFiles {
		err := ioutil.WriteFile(path.Join(sysfs, filename), body, 0644)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	return nil
}

func TestGetStdin(t *testing.T) {
	tcases := []struct {
		name        string
		stdinJSON   string
		expectedErr bool
	}{
		{
			name:        "correct stdin",
			stdinJSON:   "stdin-correct.json",
			expectedErr: false,
		},
		{
			name:        "bundle field is not set",
			stdinJSON:   "stdin-no-bundle.json",
			expectedErr: true,
		},
		{
			name:        "bundle directory doesn't exist",
			stdinJSON:   "stdin-bundle-driectory-doesnt-exist.json",
			expectedErr: true,
		},
		{
			name:        "incorrect JSON",
			stdinJSON:   "stdin-incorrect-JSON.json",
			expectedErr: true,
		},
		{
			name:        "no annotations",
			stdinJSON:   "stdin-no-annotations.json",
			expectedErr: true,
		},
		{
			name:        "annotation is not set",
			stdinJSON:   "stdin-incorrect-intel-annotation.json",
			expectedErr: true,
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
			if err != nil {
				t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
			}
			stdinJ, err := getStdin(stdin)
			if err != nil {
				if !tc.expectedErr {
					t.Errorf("unexpected error: %+v", err)
				}
			} else {
				if stdinJ.Annotations.ComIntelFpgaMode != "fpga.intel.com/region" {
					t.Errorf("incorrect annotation value: %s", stdinJ.Annotations.ComIntelFpgaMode)
				}
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	tcases := []struct {
		name        string
		configJSON  string
		expectedErr bool
	}{
		{
			name:        "correct config",
			configJSON:  "config-correct.json",
			expectedErr: false,
		},
		{
			name:        "incorrect JSON",
			configJSON:  "config-broken-json.json",
			expectedErr: true,
		},
		{
			name:        "no process key in JSON",
			configJSON:  "config-no-process.json",
			expectedErr: true,
		},
		{
			name:        "no process.env key in JSON",
			configJSON:  "config-no-env.json",
			expectedErr: true,
		},
		{
			name:        "no linux key in JSON",
			configJSON:  "config-no-linux.json",
			expectedErr: true,
		},
		{
			name:        "no linux.devices key in JSON",
			configJSON:  "config-no-devices.json",
			expectedErr: true,
		},
		{
			name:        "config file doesn't exist",
			configJSON:  "doesnt-exist",
			expectedErr: true,
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			fname := "testdata/stdin-correct.json"
			stdin, err := os.Open(fname)
			if err != nil {
				t.Fatalf("can't open file %s: %v", fname, err)
			}
			stdinJ, err := getStdin(stdin)
			if err != nil {
				t.Fatalf("can't decode %s: %+v", fname, err)
			}

			he := newHookEnv("", tc.configJSON, nil)

			config, err := he.getConfig(stdinJ)
			if err != nil {
				if !tc.expectedErr {
					t.Errorf("unexpected error: %+v", err)
				}
			} else {
				if len(config.Process.Env) == 0 {
					t.Errorf("%s: process.env is empty", tc.configJSON)
				}
				if len(config.Linux.Devices) == 0 {
					t.Errorf("%s: linux.devices is empty", tc.configJSON)
				}
			}
		})
	}
}

func TestGetFPGAParams(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/fpgacriohook-TestGetFPGAParams-%d", time.Now().Unix())
	sysfsOPAE := path.Join(tmpdir, "sys", "class", "fpga")
	// sysfsDFL := path.Join(tmpdir, "sys", "class", "fpga_region")
	tcases := []struct {
		name               string
		sysfs              string
		stdinJSON          string
		configJSON         string
		afuIDPath          string
		sysfsdirs          []string
		sysfsfiles         map[string][]byte
		expectedErr        bool
		expectedRegion     string
		expectedAFU        string
		expectedPortDevice string
	}{
		// {
		// 	name:       "correct OPAE setup",
		// 	sysfs:      sysfsOPAE,
		// 	stdinJSON:  "stdin-correct.json",
		// 	configJSON: "config-correct.json",
		// 	sysfsdirs: []string{
		// 		"intel-fpga-dev.0/intel-fpga-fme.0/pr",
		// 		"intel-fpga-dev.0/intel-fpga-port.0",
		// 	},
		// 	sysfsfiles: map[string][]byte{
		// 		"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
		// 		"intel-fpga-dev.0/intel-fpga-port.0/afu_id":         []byte("f7df405cbd7acf7222f144b0b93acd18"),
		// 		"intel-fpga-dev.0/intel-fpga-port.0/dev":            []byte("100:0"),
		// 	},
		// 	expectedErr:        false,
		// 	expectedRegion:     "ce48969398f05f33946d560708be108a",
		// 	expectedAFU:        "f7df405cbd7acf7222f144b0b93acd18",
		// 	expectedPortDevice: "intel-fpga-port.0",
		// },
		// {
		// 	name:       "correct DFL setup",
		// 	sysfs:      sysfsDFL,
		// 	stdinJSON:  "stdin-correct.json",
		// 	configJSON: "config-correct-DFL.json",
		// 	sysfsdirs: []string{
		// 		"region0/dfl-fme.0/dfl-fme-region.1/fpga_region/region1",
		// 		"region0/dfl-port.0",
		// 	},
		// 	sysfsfiles: map[string][]byte{
		// 		"region0/dfl-fme.0/dfl-fme-region.1/fpga_region/region1/compat_id": []byte("ce48969398f05f33946d560708be108a"),
		// 		"region0/dfl-port.0/afu_id":                                        []byte("f7df405cbd7acf7222f144b0b93acd18"),
		// 		"region0/dfl-port.0/dev":                                           []byte("100:0"),
		// 	},
		// 	expectedErr:        false,
		// 	expectedRegion:     "ce48969398f05f33946d560708be108a",
		// 	expectedAFU:        "f7df405cbd7acf7222f144b0b93acd18",
		// 	expectedPortDevice: "dfl-port.0",
		// },
		// {
		// 	name:       "incorrect interface id",
		// 	sysfs:      sysfsOPAE,
		// 	stdinJSON:  "stdin-correct.json",
		// 	configJSON: "config-correct.json",
		// 	sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
		// 	sysfsfiles: map[string][]byte{
		// 		"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("incorrectinterfaceuuid"),
		// 	},
		// 	expectedErr: true,
		// },
		{
			name:        "no region in config",
			sysfs:       sysfsOPAE,
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-region.json",
			expectedErr: true,
		},
		{
			name:        "no AFU in config",
			sysfs:       sysfsOPAE,
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-afu.json",
			expectedErr: true,
		},
		{
			name:        "no FPGA devices in config",
			sysfs:       sysfsOPAE,
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-FPGA-devices.json",
			expectedErr: true,
		},
		{
			name:        "region and AFU don't match",
			sysfs:       sysfsOPAE,
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-region-afu-dont-match.json",
			expectedErr: true,
		},
	}
	for tcnum, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
			if err != nil {
				t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
			}

			err = createTestDirs(tc.sysfs, tc.sysfsdirs, tc.sysfsfiles)
			if err != nil {
				t.Fatalf("can't create temp files: %+v", err)
			}

			he := newHookEnv("", tc.configJSON, nil)

			stdinJ, err := getStdin(stdin)
			if err != nil {
				t.Fatalf("can't parse stdin JSON %s: %+v", tc.stdinJSON, err)
			}

			config, err := he.getConfig(stdinJ)
			if err != nil {
				t.Fatalf("can't parse JSON config %s: %+v", tc.configJSON, err)
			}

			params, err := he.getFPGAParams(config)
			if err != nil {
				if !tc.expectedErr {
					t.Errorf("unexpected error in test case #%d: %+v", tcnum, err)
				}
			} else {
				if params[0].region != tc.expectedRegion {
					t.Errorf("#%d: expected region: %s, actual: %s", tcnum, tc.expectedRegion, params[0].region)
				} else if params[0].afu != tc.expectedAFU {
					t.Errorf("#%d: expected AFU: %s, actual: %s", tcnum, tc.expectedAFU, params[0].afu)
				} else if params[0].portDevice != tc.expectedPortDevice {
					t.Errorf("#%d: expected device node: %s, actual: %s", tcnum, tc.expectedPortDevice, params[0].portDevice)
				}
			}

			err = os.RemoveAll(tmpdir)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
