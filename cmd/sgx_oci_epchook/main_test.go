// Copyright 2021 Intel Corporation. All Rights Reserved.
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
)

func init() {
	_ = flag.Set("v", "4")
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
			stdinJSON:   "stdin-bundle-directory-doesnt-exist.json",
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
				if stdinJ.Annotations.ComIntelSgxBpfMap != "container_sgx_epc_limit" {
					t.Errorf("incorrect annotation value: %s", stdinJ.Annotations.ComIntelSgxBpfMap)
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
			name:        "no linux.cgroupsPath key in JSON",
			configJSON:  "config-no-cgroupspath.json",
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

			he := newHookEnv(tc.configJSON, false)

			config, err := he.getConfig(stdinJ)
			if err != nil {
				if !tc.expectedErr {
					t.Errorf("unexpected error: %+v", err)
				}
			} else {
				if len(config.Process.Env) == 0 {
					t.Errorf("%s: process.env is empty", tc.configJSON)
				}
				if len(config.Linux.CgroupsPath) == 0 {
					t.Errorf("%s: linux.cgroupsPath is empty", tc.configJSON)
				}
			}
		})
	}
}
