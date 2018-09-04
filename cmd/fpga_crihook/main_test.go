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
	"io/ioutil"
	"os"
	"path"
	"testing"

	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

func init() {
	debug.Activate()
}

func TestGetFPGAParams(t *testing.T) {
	tcases := []struct {
		stdinJSON         string
		configJSON        string
		afuIDPath         string
		expectedErr       bool
		expectedRegion    string
		expectedAFU       string
		expectedDeviceNum string
	}{
		{
			stdinJSON:         "stdin-correct.json",
			configJSON:        "config-correct.json",
			expectedErr:       false,
			expectedRegion:    "ce48969398f05f33946d560708be108a",
			expectedAFU:       "f7df405cbd7acf7222f144b0b93acd18",
			expectedDeviceNum: "0",
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
	for _, tc := range tcases {
		stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
		if err != nil {
			t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
		}

		content, err := decodeJSONStream(stdin)
		if err != nil {
			t.Fatalf("can't decode json file %s: %+v", tc.stdinJSON, err)
		}

		he := newHookEnv("", tc.configJSON, nil, "")

		params, err := he.getFPGAParams(content)

		if err != nil {
			if !tc.expectedErr {
				t.Errorf("unexpected error: %+v", err)
			}
		} else {
			if params.region != tc.expectedRegion {
				t.Errorf("expected region: %s, actual: %s", tc.expectedRegion, params.region)
			} else if params.afu != tc.expectedAFU {
				t.Errorf("expected AFU: %s, actual: %s", tc.expectedAFU, params.afu)
			} else if params.devNum != tc.expectedDeviceNum {
				t.Errorf("expected device number: %s, actual: %s", tc.expectedDeviceNum, params.devNum)
			}
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
		params      *fpgaParams
		expectedErr bool
		fakeAction  []fakeexec.FakeCombinedOutputAction
	}{
		{
			params: &fpgaParams{
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
			params: &fpgaParams{
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
			params: &fpgaParams{
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) { return nil, &fakeexec.FakeExitError{Status: 1} },
			},
		},
		{
			params: &fpgaParams{
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
			params: &fpgaParams{
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
			params: &fpgaParams{
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
			params: &fpgaParams{
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
			params: &fpgaParams{
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
			params: &fpgaParams{
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
			params: &fpgaParams{
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
		he := newHookEnv(fpgaBitStreamDir, "", &execer, "")
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
		params           *fpgaParams
		afuIDTemplate    string
		newAFUIDTemplate string
		expectedErr      bool
		fpgaconfErr      bool
	}{
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d7724dc4a4a3c413f89e433683f9040b"},
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d7724dc4a4a3c413f89e433683f9040b",
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7dfaaacbd7acf7222f144b0b93acd18"},
			expectedErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			expectedErr: true,
			fpgaconfErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "18b7bffa2eb54aa096ef4230dafacb5a"},
			expectedErr: true,
			fpgaconfErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/doesnt_exist",
			expectedErr:   true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			expectedErr:      true,
		},
	}

	for _, tc := range tcases {
		he := newHookEnv(fpgaBitStreamDir, "", nil, tc.afuIDTemplate)
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
	tcases := []struct {
		stdinJSON        string
		configJSON       string
		params           *fpgaParams
		afuIDTemplate    string
		newAFUIDTemplate string
		expectedErr      bool
		fpgaconfErr      bool
		gbsInfoAction    fakeexec.FakeCombinedOutputAction
	}{
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-correct.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:        "stdin-correct.json",
			configJSON:       "config-correct.json",
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_f7df405cbd7acf7222f144b0b93acd18",
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:   "stdin-broken-json.json",
			expectedErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:   "stdin-no-annotations.json",
			expectedErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:   "stdin-no-intel-annotation.json",
			expectedErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:   "stdin-incorrect-intel-annotation.json",
			expectedErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-no-afu.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-correct.json",
			expectedErr: true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-non-existing-bitstream.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-correct.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-no-accelerator-type-uuid.json")
			},
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-correct.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "d8424dc4a4a3c413f89e433683f9040b"},
			stdinJSON:     "stdin-correct.json",
			configJSON:    "config-correct.json",
			afuIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:   true,
			fpgaconfErr:   true,
			gbsInfoAction: func() ([]byte, error) {
				return ioutil.ReadFile("testdata/gbs-info-correct.json")
			},
		},
		{
			params: &fpgaParams{
				devNum: "0",
				region: "ce48969398f05f33946d560708be108a",
				afu:    "f7df405cbd7acf7222f144b0b93acd18"},
			stdinJSON:        "stdin-correct.json",
			configJSON:       "config-correct.json",
			afuIDTemplate:    "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			newAFUIDTemplate: "testdata/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id_d8424dc4a4a3c413f89e433683f9040b",
			expectedErr:      true,
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

		he := newHookEnv("testdata/intel.com/fpga", tc.configJSON, nil, tc.afuIDTemplate)

		actions := []fakeexec.FakeCombinedOutputAction{
			tc.gbsInfoAction,
			genFpgaConfAction(he, tc.newAFUIDTemplate, tc.fpgaconfErr),
		}
		fcmd := fakeexec.FakeCmd{CombinedOutputScript: actions}
		he.execer = &fakeexec.FakeExec{CommandScript: genFakeActions(&fcmd, len(fcmd.CombinedOutputScript))}

		err = he.process(stdin)

		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: %+v", err)
		}
	}
}
