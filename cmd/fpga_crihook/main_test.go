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
	"flag"
	"os"
	"path"
	"testing"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/bitstream"
	"github.com/pkg/errors"
)

func init() {
	_ = flag.Set("v", "4")
}

func createTestDirs(sysfs string, sysfsDirs []string, sysfsFiles map[string][]byte) error {
	for _, sysfsdir := range sysfsDirs {
		err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0750)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}

	for filename, body := range sysfsFiles {
		err := os.WriteFile(path.Join(sysfs, filename), body, 0600)
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

			he := newHookEnv("", tc.configJSON, fpga.NewPort)

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

// testFpgaPort represent Fake FPGA Port device for testing purposes.
type testFpgaPort struct {
	fpga.Port
	interfaceUUIDS  []string
	accelTypeUUIDS  []string
	callNo          int
	failProgramming bool
}

// GetInterfaceUUID returns Interface UUID.
func (p *testFpgaPort) GetInterfaceUUID() (id string) {
	uuid := p.interfaceUUIDS[p.callNo]
	p.callNo++

	return uuid
}

// GetAcceleratorTypeUUID returns AFU UUID.
func (p *testFpgaPort) GetAcceleratorTypeUUID() string {
	uuid := p.accelTypeUUIDS[p.callNo]
	p.callNo++

	return uuid
}

// PR fakes programming specified bitstream.
func (p *testFpgaPort) PR(bs bitstream.File, dryRun bool) error {
	if p.failProgramming {
		return errors.New("fail to program device")
	}

	return nil
}

func newTestPort(dev string) (fpga.Port, error) {
	return &testFpgaPort{
		interfaceUUIDS: []string{"ce48969398f05f33946d560708be108a"},
		accelTypeUUIDS: []string{"d8424dc4a4a3c413f89e433683f9040b"},
	}, nil
}

func TestGetFPGAParams(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "TestGetFPGAParams")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(tmpdir)

	sysfsTest := path.Join(tmpdir, "sys", "class", "fpga")

	tcases := []struct {
		name               string
		sysfs              string
		stdinJSON          string
		configJSON         string
		afuIDPath          string
		expectedRegion     string
		expectedAFU        string
		expectedPortDevice string
		sysfsfiles         map[string][]byte
		sysfsdirs          []string
		expectedErr        bool
	}{
		{
			name:       "valid setup",
			sysfs:      sysfsTest,
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs: []string{
				"intel-fpga-dev.0/test-fpga-fme.0/pr",
				"intel-fpga-dev.0/test-fpga-port.0",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/test-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
				"intel-fpga-dev.0/test-fpga-port.0/afu_id":         []byte("f7df405cbd7acf7222f144b0b93acd18"),
				"intel-fpga-dev.0/test-fpga-port.0/dev":            []byte("100:0"),
			},
			expectedErr:        false,
			expectedRegion:     "ce48969398f05f33946d560708be108a",
			expectedAFU:        "f7df405cbd7acf7222f144b0b93acd18",
			expectedPortDevice: "intel-fpga-port.0",
		},
		{
			name:        "no region in config",
			sysfs:       sysfsTest,
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-region.json",
			expectedErr: true,
		},
		{
			name:        "no AFU in config",
			sysfs:       sysfsTest,
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-afu.json",
			expectedErr: true,
		},
		{
			name:        "no FPGA devices in config",
			sysfs:       sysfsTest,
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-FPGA-devices.json",
			expectedErr: true,
		},
		{
			name:        "region and AFU don't match",
			sysfs:       sysfsTest,
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

			he := newHookEnv("", tc.configJSON, newTestPort)

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

func TestProcess(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "testProcess")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(tmpdir)

	sysfs := path.Join(tmpdir, "sys", "class", "fpga")
	tcases := []struct {
		name        string
		stdinJSON   string
		configJSON  string
		newPort     newPortFun
		sysfsfiles  map[string][]byte
		sysfsdirs   []string
		expectedErr bool
	}{
		{
			name:       "Reprogramming",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			newPort: func(dev string) (fpga.Port, error) {
				return &testFpgaPort{
					interfaceUUIDS: []string{"ce48969398f05f33946d560708be108a"},
					accelTypeUUIDS: []string{
						"d8424dc4a4a3c413f89e433683f9040b",
						"f7df405cbd7acf7222f144b0b93acd18"},
				}, nil
			},
		},
		{
			name:        "Broken stdin",
			stdinJSON:   "stdin-broken-json.json",
			expectedErr: true,
		},
		{
			name:        "Broken config",
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-broken-json.json",
			expectedErr: true,
		},
		{
			name:        "Failing to get FPGA params",
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-region.json",
			expectedErr: true,
		},
		{
			name:       "Already programmed",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			newPort: func(dev string) (fpga.Port, error) {
				return &testFpgaPort{
					interfaceUUIDS: []string{"ce48969398f05f33946d560708be108a"},
					accelTypeUUIDS: []string{"f7df405cbd7acf7222f144b0b93acd18"},
				}, nil
			},
		},
		{
			name:       "Non-existing bitstream",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-non-existing-bitstream.json",
			newPort: func(dev string) (fpga.Port, error) {
				return &testFpgaPort{
					interfaceUUIDS: []string{"ce48969398f05f33946d560708be108a"},
					accelTypeUUIDS: []string{
						"d8424dc4a4a3c413f89e433683f9040b",
						"f7df405cbd7acf7222f144b0b93acd18"},
				}, nil
			},
			expectedErr: true,
		},
		{
			name:       "Programming fails",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			newPort: func(dev string) (fpga.Port, error) {
				return &testFpgaPort{
					interfaceUUIDS: []string{"ce48969398f05f33946d560708be108a"},
					accelTypeUUIDS: []string{
						"d8424dc4a4a3c413f89e433683f9040b",
						"f7df405cbd7acf7222f144b0b93acd18"},
					failProgramming: true,
				}, nil
			},
			expectedErr: true,
		},
		{
			name:       "Device is not reprogrammed",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			newPort: func(dev string) (fpga.Port, error) {
				return &testFpgaPort{
					interfaceUUIDS: []string{"ce48969398f05f33946d560708be108a"},
					accelTypeUUIDS: []string{
						"d8424dc4a4a3c413f89e433683f9040b",
						"d8424dc4a4a3c413f89e433683f9040b"},
				}, nil
			},
			expectedErr: true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
			if err != nil {
				t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
			}

			err = createTestDirs(sysfs, tc.sysfsdirs, tc.sysfsfiles)
			if err != nil {
				t.Fatalf("can't create temp files: %+v", err)
			}

			he := newHookEnv("testdata/intel.com/fpga", tc.configJSON, tc.newPort)

			err = he.process(stdin)

			if err != nil && !tc.expectedErr {
				t.Errorf("[%s]: unexpected error: %+v", tc.name, err)
			}

			err = os.RemoveAll(tmpdir)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
