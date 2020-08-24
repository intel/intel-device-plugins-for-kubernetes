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
	"flag"
	"io/ioutil"
	"os"
	"path"
	"testing"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

func init() {
	flag.Set("v", "4") // Enable debug output
}

// mockNotifier implements Notifier interface.
type mockNotifier struct {
	scanDone          chan bool
	enclaveDevCount   int
	provisionDevCount int
}

// Notify stops plugin Scan.
func (n *mockNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.enclaveDevCount = len(newDeviceTree[deviceTypeEnclave])
	n.provisionDevCount = len(newDeviceTree[deviceTypeProvision])
	n.scanDone <- true
}

func TestPodCount(t *testing.T) {
	tcases := []struct {
		name             string
		envValue         string
		nCPUs            uint
		expectedPodCount uint
	}{
		{
			name:             "Only CPU count",
			envValue:         "",
			nCPUs:            5,
			expectedPodCount: defaultPodsPerCore * 5,
		},
		{
			name:             "Broken ENV",
			envValue:         "foobar",
			nCPUs:            5,
			expectedPodCount: defaultPodsPerCore * 5,
		},
		{
			name:             "Valid ENV",
			envValue:         "2200",
			nCPUs:            5,
			expectedPodCount: 2200 * 5,
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			os.Unsetenv(podsPerCoreEnvVariable)
			if tc.envValue != "" {
				os.Setenv(podsPerCoreEnvVariable, tc.envValue)
			}

			count := getDefaultPodCount(tc.nCPUs)

			if tc.expectedPodCount != count {
				t.Errorf("Wrong value for expected pod count")
			}
		})
	}
}

func TestScan(t *testing.T) {
	tcases := []struct {
		name                   string
		enclaveDevice          string
		provisionDevice        string
		requestedEnclaveDevs   uint
		requestedProvisionDevs uint
		expectedEnclaveDevs    int
		expectedProvisionDevs  int
	}{
		{
			name: "no device installed",
		},
		{
			name:                  "only enclave file",
			enclaveDevice:         "enclave",
			requestedEnclaveDevs:  1,
			expectedEnclaveDevs:   0,
			expectedProvisionDevs: 0,
		},
		{
			name:                   "only provision file",
			provisionDevice:        "provision",
			requestedProvisionDevs: 1,
			expectedEnclaveDevs:    0,
			expectedProvisionDevs:  0,
		},
		{
			name:                   "one device",
			enclaveDevice:          "enclave",
			provisionDevice:        "provision",
			requestedEnclaveDevs:   1,
			expectedEnclaveDevs:    1,
			requestedProvisionDevs: 1,
			expectedProvisionDevs:  1,
		},
		{
			name:                   "one device",
			enclaveDevice:          "enclave",
			provisionDevice:        "provision",
			requestedEnclaveDevs:   10,
			expectedEnclaveDevs:    10,
			requestedProvisionDevs: 20,
			expectedProvisionDevs:  20,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := ioutil.TempDir("", "test_sgx_device_plugin_")
			if err != nil {
				t.Fatalf("can't create temporary directory: %+v", err)
			}
			defer os.RemoveAll(root)

			devfs := path.Join(root, "dev")
			err = os.MkdirAll(path.Join(devfs, "sgx"), 0755)
			if err != nil {
				t.Fatalf("Failed to create fake device directory: %+v", err)
			}
			if tc.enclaveDevice != "" {
				err = ioutil.WriteFile(path.Join(devfs, "sgx", tc.enclaveDevice), []byte{}, 0600)
				if err != nil {
					t.Fatalf("Failed to create fake vendor file: %+v", err)
				}
			}
			if tc.provisionDevice != "" {
				err = ioutil.WriteFile(path.Join(devfs, "sgx", tc.provisionDevice), []byte{}, 0600)
				if err != nil {
					t.Fatalf("Failed to create fake vendor file: %+v", err)
				}
			}

			plugin := newDevicePlugin(devfs, tc.requestedEnclaveDevs, tc.requestedProvisionDevs)

			notifier := &mockNotifier{
				scanDone: plugin.scanDone,
			}

			err = plugin.Scan(notifier)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tc.expectedEnclaveDevs != notifier.enclaveDevCount {
				t.Errorf("Wrong number of discovered enclave devices")
			}
			if tc.expectedProvisionDevs != notifier.provisionDevCount {
				t.Errorf("Wrong number of discovered provision devices")
			}
		})
	}
}
