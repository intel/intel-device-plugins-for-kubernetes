// Copyright 2019 Intel Corporation. All Rights Reserved.
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

package bitstream

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/pkg/errors"
	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

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

func TestGetFPGABitstream(t *testing.T) {
	var fpgaBitStreamDir = "testdata/intel.com/fpga"

	tcases := []struct {
		name         string
		bitstreamDir string
		region       string
		afu          string
		expectedErr  bool
	}{
		{
			name:         "Get broken OPAE bistream file",
			bitstreamDir: fpgaBitStreamDir,
			region:       "ce48969398f05f33946d560708be108a",
			afu:          "f7df405cbd7acf7222f144b0b93acd18",
			expectedErr:  true,
		},
		{
			name:         "Get broken OpenCL bistream file",
			bitstreamDir: fpgaBitStreamDir,
			region:       "ce48969398f05f33946d560708be108a",
			afu:          "d7724dc4a4a3c413f89e433683f9040b",
			expectedErr:  true,
		},
		{
			name:         "Bitstream not found",
			bitstreamDir: fpgaBitStreamDir,
			region:       "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			afu:          "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy",
			expectedErr:  true,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GetFPGABitstream(tc.bitstreamDir, tc.region, tc.afu)
			if err != nil {
				if !tc.expectedErr {
					t.Errorf("unexpected error ocurred: %+v", err)
				}
				return
			}
		})
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
