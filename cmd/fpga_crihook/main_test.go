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

	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"

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

func TestGetFPGAParams(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/fpgacriohook-TestGetFPGAParams-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sys", "class", "fpga")
	tcases := []struct {
		stdinJSON         string
		configJSON        string
		afuIDPath         string
		sysfsdirs         []string
		sysfsfiles        map[string][]byte
		expectedErr       bool
		expectedRegion    string
		expectedAFU       string
		expectedDeviceNum string
	}{
		{
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			expectedErr:       false,
			expectedRegion:    "ce48969398f05f33946d560708be108a",
			expectedAFU:       "f7df405cbd7acf7222f144b0b93acd18",
			expectedDeviceNum: "0",
		},
		{
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("incorrectinterfaceuuid"),
			},
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-no-bundle.json",
			configJSON:  "config-correct.json",
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-bundle-dir-doesnt-exist.json",
			configJSON:  "config-correct.json",
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-broken-json.json",
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-process.json",
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-env.json",
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-region.json",
			expectedErr: true,
		},
		{
			stdinJSON:      "stdin-correct.json",
			configJSON:     "config-no-afu.json",
			expectedErr:    true,
			expectedRegion: "ce48969398f05f33946d560708be108a",
		},
		{
			stdinJSON:      "stdin-correct.json",
			configJSON:     "config-no-linux.json",
			expectedErr:    true,
			expectedRegion: "ce48969398f05f33946d560708be108a",
			expectedAFU:    "f7df405cbd7acf7222f144b0b93acd18",
		},
		{
			stdinJSON:      "stdin-correct.json",
			configJSON:     "config-no-devices.json",
			expectedErr:    true,
			expectedRegion: "ce48969398f05f33946d560708be108a",
			expectedAFU:    "f7df405cbd7acf7222f144b0b93acd18",
		},
		{
			stdinJSON:      "stdin-correct.json",
			configJSON:     "config-no-FPGA-devices.json",
			expectedErr:    true,
			expectedRegion: "ce48969398f05f33946d560708be108a",
			expectedAFU:    "f7df405cbd7acf7222f144b0b93acd18",
		},
	}
	for tcnum, tc := range tcases {
		stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
		if err != nil {
			t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
		}

		err = createTestDirs(sysfs, tc.sysfsdirs, tc.sysfsfiles)
		if err != nil {
			t.Fatalf("can't create temp files: %+v", err)
		}

		content, err := decodeJSONStream(stdin)
		if err != nil {
			t.Fatalf("can't decode json file %s: %+v", tc.stdinJSON, err)
		}

		he := newHookEnv("", tc.configJSON, nil, "", path.Join(sysfs, "intel-fpga-dev.%s/intel-fpga-fme.%s/pr/interface_id"))

		params, err := he.getFPGAParams(content)

		if err != nil {
			if !tc.expectedErr {
				t.Errorf("unexpected error in test case #%d: %+v", tcnum, err)
			}
		} else {
			if params[0].region != tc.expectedRegion {
				t.Errorf("#%d: expected region: %s, actual: %s", tcnum, tc.expectedRegion, params[0].region)
			} else if params[0].afu != tc.expectedAFU {
				t.Errorf("#%d: expected AFU: %s, actual: %s", tcnum, tc.expectedAFU, params[0].afu)
			} else if params[0].devNum != tc.expectedDeviceNum {
				t.Errorf("#%d: expected device number: %s, actual: %s", tcnum, tc.expectedDeviceNum, params[0].devNum)
			}
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func genFakeActions(fcmd *fakeexec.FakeCmd, num int) []fakeexec.FakeCommandAction {
	var actions []fakeexec.FakeCommandAction
	for i := 0; i < num; i++ {
		actions = append(actions, func(cmd string, args ...string) exec.Cmd {
			return fakeexec.InitFakeCmd(fcmd, cmd, args...)
		})
	}
	return actions
}

func TestValidate(t *testing.T) {
	var fpgaBitStreamDir = "testdata/intel.com/fpga"
	tcases := []struct {
		params      fpgaParams
		expectedErr bool
		fakeAction  []fakeexec.FakeCombinedOutputAction
	}{
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: false,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-correct.json")
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d7724dc4a4a3c413f89e433683f9040b"},
			expectedErr: false,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return nil, nil
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) { return nil, &fakeexec.FakeExitError{Status: 1} },
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-broken-json.json")
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-afu-image.json")
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-interface-uuid.json")
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-accelerator-clusters.json")
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05fxxxxxxxxxxxxxxxxxx",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-correct.json")
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-accelerator-type-uuid.json")
				},
			},
		},
		{
			params: fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-correct.json")
				},
			},
		},
	}

	for _, tc := range tcases {
		fcmd := fakeexec.FakeCmd{CombinedOutputScript: tc.fakeAction}
		execer := fakeexec.FakeExec{CommandScript: genFakeActions(&fcmd, len(fcmd.CombinedOutputScript))}
		he := newHookEnv(fpgaBitStreamDir, "", &execer, "", "")
		bitStream, err := he.getBitStream(tc.params)
		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: unable to get bitstream: %+v", err)
			continue
		}
		err = bitStream.validate()
		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: bitstream validation failed: %+v", err)
		}
	}
}

func genFpgaConfAction(he *hookEnv, afuIDTemplate string, returnError bool) fakeexec.FakeCombinedOutputAction {
	return func() ([]byte, error) {
		if returnError {
			return []byte("error"), &fakeexec.FakeExitError{Status: 1}
		}
		he.afuIDTemplate = afuIDTemplate // emulate reprogramming
		return []byte(""), nil
	}
}

func TestProgram(t *testing.T) {
	var fpgaBitStreamDir = "testdata/intel.com/fpga"
	tcases := []struct {
		params           fpgaParams
		afuIDTemplate    string
		newAFUIDTemplate string
		expectedErr      bool
		fpgaconfErr      bool
	}{
		{
			params: fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
		},
		{
			params: fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d7724dc4a4a3c413f89e433683f9040b"},
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d7724dc4a4a3c413f89e433683f9040b",
		},
		{
			params: fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7dfaaacbd7acf7222f144b0b93acd18"},
			expectedErr: true,
		},
		{
			params: fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fpgaconfErr: true,
		},
		{
			params: fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "18b7bffa2eb54aa096ef4230dafacb5a"},
			expectedErr: true,
			fpgaconfErr: true,
		},
		{
			params: fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/doesnt_exist",
			expectedErr:   true,
		},
		{
			params: fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			expectedErr:      true,
		},
	}

	for _, tc := range tcases {
		he := newHookEnv(fpgaBitStreamDir, "", nil, tc.afuIDTemplate, "")
		actions := []fakeexec.FakeCombinedOutputAction{genFpgaConfAction(he, tc.newAFUIDTemplate, tc.fpgaconfErr)}
		fcmd := fakeexec.FakeCmd{CombinedOutputScript: actions}
		he.execer = &fakeexec.FakeExec{CommandScript: genFakeActions(&fcmd, len(fcmd.CombinedOutputScript))}
		bitStream, err := he.getBitStream(tc.params)
		if err != nil {
			if !tc.expectedErr {
				t.Errorf("unexpected error: unable to get bitstream: %+v", err)
			}
			continue
		}
		err = bitStream.program()
		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: programming bitstream failed: %+v", err)
		}
	}
}

func TestProcess(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/fpgacriohook-TestProcess-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sys", "class", "fpga")
	tcases := []struct {
		name             string
		stdinJSON        string
		configJSON       string
		afuIDTemplate    string
		newAFUIDTemplate string
		sysfsdirs        []string
		sysfsfiles       map[string][]byte
		expectedErr      bool
		fpgaconfErr      bool
		gbsInfoAction    fakeexec.FakeCombinedOutputAction
	}{
		{
			name:       "All correct #1",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			name:       "All correct #2",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			name:        "Broken stdin json",
			stdinJSON:   "stdin-broken-json.json",
			expectedErr: true,
		},
		{
			name:        "No annotations in stdin at all",
			stdinJSON:   "stdin-no-annotations.json",
			expectedErr: true,
		},
		{
			name:        "No Intel annotations in stdin",
			stdinJSON:   "stdin-no-intel-annotation.json",
			expectedErr: true,
		},
		{
			name:        "Incorrect annotations in stdin",
			stdinJSON:   "stdin-incorrect-intel-annotation.json",
			expectedErr: true,
		},
		{
			name:          "No AFU ID present in container config",
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-no-afu.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
		},
		{
			name:       "Unverifyable info",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			expectedErr: true,
		},
		{
			name:          "Non-existing bitstream present in container config",
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-non-existing-bitstream.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
		},
		{
			name:       "No accelerator type UUID present in GBS",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-no-accelerator-type-uuid.json")
			},
		},
		{
			name:       "Mismatching bitsream",
			stdinJSON:  "stdin-correct.json",
			configJSON: "config-correct.json",
			sysfsdirs:  []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			name:          "Processing error #1",
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-correct.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			sysfsdirs:     []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			expectedErr: true,
			fpgaconfErr: true,
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			name:             "Processing error #2",
			stdinJSON:        "stdin-correct.json",
			configJSON:       "config-correct.json",
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			sysfsdirs:        []string{"intel-fpga-dev.0/intel-fpga-fme.0/pr"},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a"),
			},
			expectedErr: true,
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
	}

	for _, tc := range tcases {
		stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
		if err != nil {
			t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
		}

		err = createTestDirs(sysfs, tc.sysfsdirs, tc.sysfsfiles)
		if err != nil {
			t.Fatalf("can't create temp files: %+v", err)
		}

		he := newHookEnv("testdata/intel.com/fpga", tc.configJSON, nil, tc.afuIDTemplate, path.Join(sysfs, "intel-fpga-dev.%s/intel-fpga-fme.%s/pr/interface_id"))

		actions := []fakeexec.FakeCombinedOutputAction{
			tc.gbsInfoAction,
			genFpgaConfAction(he, tc.newAFUIDTemplate, tc.fpgaconfErr),
		}
		fcmd := fakeexec.FakeCmd{CombinedOutputScript: actions}
		he.execer = &fakeexec.FakeExec{CommandScript: genFakeActions(&fcmd, len(fcmd.CombinedOutputScript))}

		err = he.process(stdin)

		if err != nil && !tc.expectedErr {
			t.Errorf("[%s]: unexpected error: %+v", tc.name, err)
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatal(err)
		}
	}
}
