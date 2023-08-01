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

package topology

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	topo "github.com/containers/nri-plugins/pkg/topology"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func setupTestEnv(t *testing.T) func() {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal("unable to get current directory")
	}

	if path, err := filepath.EvalSymlinks(pwd); err == nil {
		pwd = path
	}

	topo.SetSysRoot(pwd + "/testdata")

	teardown := func() {
		topo.SetSysRoot("")
	}

	return teardown
}

func TestGetTopologyInfo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	teardown := setupTestEnv(t)
	defer teardown()

	cases := []struct {
		name        string
		output      *pluginapi.TopologyInfo
		input       []string
		expectedErr bool
	}{
		{
			name:  "valid: device with 3 numa nodes",
			input: []string{"/dev/null"},
			output: &pluginapi.TopologyInfo{
				Nodes: []*pluginapi.NUMANode{
					{ID: 1},
					{ID: 2},
					{ID: 3},
				},
			},
			expectedErr: false,
		},
		{
			name:  "valid: 2 identical devices with 3 numa nodes",
			input: []string{"/dev/null", "/dev/null"},
			output: &pluginapi.TopologyInfo{
				Nodes: []*pluginapi.NUMANode{
					{ID: 1},
					{ID: 2},
					{ID: 3},
				},
			},
			expectedErr: false,
		},
		{
			name:  "valid: 2 different devicees with 3 numa nodes each",
			input: []string{"/dev/tty", "/dev/null"},
			output: &pluginapi.TopologyInfo{
				Nodes: []*pluginapi.NUMANode{
					{ID: 1},
					{ID: 2},
					{ID: 3},
					{ID: 4},
					{ID: 5},
					{ID: 6},
				},
			},
			expectedErr: false,
		},
		{
			name:        "invalid: device doesn't exist",
			input:       []string{"/dev/non-existing-device"},
			output:      nil,
			expectedErr: true,
		},
		{
			name:        "invalid: empty device path",
			input:       []string{""},
			output:      nil,
			expectedErr: true,
		},
		{
			name:        "valid: missing numa node ID",
			input:       []string{"/dev/random"},
			output:      &pluginapi.TopologyInfo{},
			expectedErr: false,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			output, err := GetTopologyInfo(test.input)
			switch {
			case err != nil && !test.expectedErr:
				t.Fatalf("unexpected error returned: %+v", err)
			case err == nil && test.expectedErr:
				t.Fatalf("unexpected success: %+v", output)
			case !reflect.DeepEqual(output, test.output):
				t.Fatalf("expected: %q got: %q", test.output, output)
			}
		})
	}
}
