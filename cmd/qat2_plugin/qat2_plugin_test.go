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
	"os"
	"path"
	"testing"
	"time"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

const (
	adfCtlOutput = `Checking status of all devices.
There is 3 QAT acceleration device(s) in the system:
 qat_dev0 - type: c6xx,  inst_id: 0,  node_id: 0,  bsf: 3b:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev1 - type: c6xx,  inst_id: 1,  node_id: 0,  bsf: 3d:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev2 - type: c6xx,  inst_id: 2,  node_id: 3,  bsf: d8:00.0,  #accel: 5 #engines: 10 state: up
`
)

func init() {
	debug.Activate()
}

func TestParseConfigs(t *testing.T) {
	tcases := []struct {
		name        string
		testData    string
		expectedErr bool
	}{
		{
			name:     "All is good",
			testData: "all_is_good",
		},
		{
			name:        "Missing section with LinitDevAccess=1",
			testData:    "missing_pinned_section",
			expectedErr: true,
		},
	}
	for _, tc := range tcases {
		fcmd := fakeexec.FakeCmd{
			CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
				func() ([]byte, error) {
					return []byte(adfCtlOutput), nil
				},
			},
		}
		execer := fakeexec.FakeExec{
			CommandScript: []fakeexec.FakeCommandAction{
				func(cmd string, args ...string) exec.Cmd {
					return fakeexec.InitFakeCmd(&fcmd, cmd, args...)
				},
			},
		}
		dp := &devicePlugin{
			execer:    &execer,
			configDir: "./test_data/" + tc.testData,
		}
		_, err := dp.parseConfigs()
		if tc.expectedErr && err == nil {
			t.Errorf("Test case '%s': expected error hasn't been triggered", tc.name)
		}
		if !tc.expectedErr && err != nil {
			t.Errorf("Test case '%s': Unexpected error: %+v", tc.name, err)
		}
	}
}

func TestGetDevTree(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/qatplugin-getDevTree-%d", time.Now().Unix())
	tcases := []struct {
		name        string
		devfs       string
		uiodevs     []string
		config      map[string]section
		expectedErr bool
	}{
		{
			name:    "All is good",
			devfs:   "dev",
			uiodevs: []string{"uio4", "uio5"},
			config: map[string]section{
				"TESTSHIM": {
					endpoints: []endpoint{
						{
							id:        "dev0",
							processes: 2,
						},
					},
				},
				"TESTSHIM2": {
					endpoints: []endpoint{
						{
							id:        "dev0",
							processes: 2,
						},
					},
				},
				"TESTPINNED": {
					endpoints: []endpoint{
						{
							id:        "dev0",
							processes: 2,
						},
					},
					pinned: true,
				},
			},
		},
		{
			name:        "Wrong devfs",
			devfs:       "wrongdev",
			expectedErr: true,
		},
	}
	for _, tc := range tcases {
		var err error

		devfs := path.Join(tmpdir, "dev")
		err = os.MkdirAll(devfs, 0755)
		if err != nil {
			t.Fatal(err)
		}

		for _, uiodev := range tc.uiodevs {
			err = os.MkdirAll(path.Join(devfs, uiodev), 0755)
			if err != nil {
				t.Fatal(err)
			}
		}

		_, err = getDevTree(path.Join(tmpdir, tc.devfs), tc.config)
		if tc.expectedErr && err == nil {
			t.Errorf("Test case '%s': expected error hasn't been triggered", tc.name)
		}
		if !tc.expectedErr && err != nil {
			t.Errorf("Test case '%s': Unexpected error: %+v", tc.name, err)
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestPostAllocate(t *testing.T) {
	tcases := []struct {
		name         string
		envs         map[string]string
		expectedEnvs []string
		expectedErr  bool
	}{
		{
			name: "All is good",
			envs: map[string]string{
				"SOMEVAR":                     "some value",
				"QAT_SECTION_NAME_cy1_dc0_15": "TESTSHIM",
				"QAT_SECTION_NAME_cy1_dc0_32": "TESTSHIM2",
			},
			expectedEnvs: []string{
				"SOMEVAR",
				"QAT_SECTION_NAME_cy1_dc0_0",
				"QAT_SECTION_NAME_cy1_dc0_1",
			},
		},
		{
			name: "Wrong env variable name format",
			envs: map[string]string{
				"QAT_SECTION_NAME_JUSTWRONG": "some value",
			},
			expectedErr: true,
		},
	}
	for _, tc := range tcases {
		response := new(pluginapi.AllocateResponse)
		cresp := new(pluginapi.ContainerAllocateResponse)
		cresp.Envs = tc.envs
		response.ContainerResponses = append(response.ContainerResponses, cresp)

		dp := &devicePlugin{}

		err := dp.PostAllocate(response)

		for _, key := range tc.expectedEnvs {
			if _, ok := cresp.Envs[key]; !ok {
				t.Errorf("Test case '%s': expcted env variable '%s' is missing", tc.name, key)
			}
		}

		if tc.expectedErr && err == nil {
			t.Errorf("Test case '%s': expected error hasn't been triggered", tc.name)
		}
		if !tc.expectedErr && err != nil {
			t.Errorf("Test case '%s': Unexpected error: %+v", tc.name, err)
		}
		debug.Print(response)
	}
}
