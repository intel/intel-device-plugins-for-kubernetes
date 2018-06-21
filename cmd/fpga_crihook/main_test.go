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
)

func TestGetFPGAParams(t *testing.T) {
	tcases := []struct {
		stdinJSON      string
		configJSON     string
		expectedErr    bool
		expectedRegion string
		expectedAFU    string
	}{
		{
			stdinJSON:      "stdin-correct.json",
			configJSON:     "config-correct.json",
			expectedErr:    false,
			expectedRegion: "ce48969398f05f33946d560708be108a",
			expectedAFU:    "f7df405cbd7acf7222f144b0b93acd18",
		},
		{
			stdinJSON:   "stdin-broken-json.json",
			configJSON:  "config-correct.json",
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
	}
	for _, tc := range tcases {
		stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
		if err != nil {
			t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
		}

		he := newHookEnv("", tc.configJSON, nil)

		region, afu, err := he.getFPGAParams(stdin)

		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: %v", err)
		} else if region != tc.expectedRegion {
			t.Errorf("expected region: %s, actual: %s", tc.expectedRegion, region)
		} else if afu != tc.expectedAFU {
			t.Errorf("expected AFU: %s, actual: %s", tc.expectedAFU, afu)
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

func TestValidateBitstream(t *testing.T) {
	tcases := []struct {
		fpgaRegion  string
		fpgaAfu     string
		expectedErr bool
		fakeAction  []fakeexec.FakeCombinedOutputAction
	}{
		{
			fpgaRegion:  "ce48969398f05f33946d560708be108a",
			fpgaAfu:     "f7df405cbd7acf7222f144b0b93acd18",
			expectedErr: false,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-correct.json")
				},
			},
		},
		{
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) { return nil, &fakeexec.FakeExitError{Status: 1} },
			},
		},
		{
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-broken-json.json")
				},
			},
		},
		{
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-afu-image.json")
				},
			},
		},
		{
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-interface-uuid.json")
				},
			},
		},
		{
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-accelerator-clusters.json")
				},
			},
		},
		{
			fpgaRegion:  "this should not match",
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-correct.json")
				},
			},
		},
		{
			fpgaRegion:  "ce48969398f05f33946d560708be108a",
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-accelerator-type-uuid.json")
				},
			},
		},
		{
			fpgaRegion:  "ce48969398f05f33946d560708be108a",
			fpgaAfu:     "this should not match",
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
		he := newHookEnv("", "", &execer)
		err := he.validateBitStream(tc.fpgaRegion, tc.fpgaAfu, "")
		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestProgramBitStream(t *testing.T) {
	tcases := []struct {
		fpgaRegion  string
		fpgaAfu     string
		expectedErr bool
		fakeAction  []fakeexec.FakeCombinedOutputAction
	}{
		{
			fpgaRegion:  "ce48969398f05f33946d560708be108a",
			fpgaAfu:     "f7df405cbd7acf7222f144b0b93acd18",
			expectedErr: false,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) { return []byte(""), nil },
			},
		},
		{
			fpgaRegion:  "ce48969398f05f33946d560708be108a",
			fpgaAfu:     "f7df405cbd7acf7222f144b0b93acd18",
			expectedErr: true,
			fakeAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) { return []byte("error"), &fakeexec.FakeExitError{Status: 1} },
			},
		},
	}

	for _, tc := range tcases {
		fcmd := fakeexec.FakeCmd{CombinedOutputScript: tc.fakeAction}
		execer := fakeexec.FakeExec{CommandScript: genFakeActions(&fcmd, len(fcmd.CombinedOutputScript))}
		he := newHookEnv("", "", &execer)
		err := he.programBitStream(tc.fpgaRegion, tc.fpgaAfu, "")
		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestProcess(t *testing.T) {
	tcases := []struct {
		stdinJSON                string
		configJSON               string
		expectedErr              bool
		fakeCombinedOutputAction []fakeexec.FakeCombinedOutputAction
	}{
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-correct.json",
			expectedErr: false,
			fakeCombinedOutputAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-correct.json")
				},
				func() ([]byte, error) { return []byte(""), nil },
			},
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-no-afu.json",
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-non-existing-bitstream.json",
			expectedErr: true,
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-correct.json",
			expectedErr: true,
			fakeCombinedOutputAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-no-accelerator-type-uuid.json")
				},
			},
		},
		{
			stdinJSON:   "stdin-correct.json",
			configJSON:  "config-correct.json",
			expectedErr: true,
			fakeCombinedOutputAction: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return ioutil.ReadFile("testdata/gbs-info-correct.json")
				},
				func() ([]byte, error) { return []byte("error"), &fakeexec.FakeExitError{Status: 1} },
			},
		},
	}

	for _, tc := range tcases {
		stdin, err := os.Open(path.Join("testdata", tc.stdinJSON))
		if err != nil {
			t.Fatalf("can't open file %s: %v", tc.stdinJSON, err)
		}

		fcmd := fakeexec.FakeCmd{CombinedOutputScript: tc.fakeCombinedOutputAction}
		execer := fakeexec.FakeExec{CommandScript: genFakeActions(&fcmd, len(fcmd.CombinedOutputScript))}

		he := newHookEnv("testdata/intel.com/fpga", tc.configJSON, &execer)
		err = he.process(stdin)

		if err != nil && !tc.expectedErr {
			t.Errorf("unexpected error: %v", err)
		}
	}
}
