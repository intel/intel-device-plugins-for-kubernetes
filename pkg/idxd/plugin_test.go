// Copyright 2017 Intel Corporation. All Rights Reserved.
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

package idxd

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	dsaMajor int = 375
)

func getFakeDevNodes(devDir, charDevDir, wqName string) ([]pluginapi.DeviceSpec, error) {
	devPath := path.Join(devDir, wqName)
	var devNum int
	var queueNum int
	fmt.Sscanf(wqName, "wq%d.%d", &devNum, &queueNum)
	charDevPath := path.Join(charDevDir, fmt.Sprintf("%d:%d", dsaMajor, devNum*10+queueNum))

	return []pluginapi.DeviceSpec{
		{
			HostPath:      devPath,
			ContainerPath: devPath,
			Permissions:   "rw",
		},
		{
			HostPath:      charDevPath,
			ContainerPath: charDevPath,
			Permissions:   "rw",
		}}, nil
}

func init() {
	_ = flag.Set("v", "4") //Enable debug output
}

// fakeNotifier implements Notifier interface.
type fakeNotifier struct {
	scanDone   chan bool
	deviceTree dpapi.DeviceTree
}

// Notify stops plugin Scan.
func (n *fakeNotifier) Notify(deviceTree dpapi.DeviceTree) {
	n.deviceTree = deviceTree
	n.scanDone <- true
}

type testCase struct {
	name           string
	sysfsDirs      []string
	sysfsFiles     map[string][]byte
	sharedDevNum   int
	expectedResult map[string]int
	expectedError  bool
}

func TestScan(t *testing.T) {
	root, err := ioutil.TempDir("", "test_idxd_device_plugin")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	sysfs := path.Join(root, "sys/bus/dsa/devices")
	statePattern := path.Join(sysfs, "dsa*/wq*/state")

	tcases := []testCase{
		{
			name: "no sysfs mounted",
		},
		{
			name:      "all queues are disabled",
			sysfsDirs: []string{"dsa0/wq0.0", "dsa0/wq0.1", "dsa0/wq0.2", "dsa0/wq0.3"},
			sysfsFiles: map[string][]byte{
				"dsa0/wq0.0/state": []byte(""),
				"dsa0/wq0.1/state": []byte(""),
				"dsa0/wq0.2/state": []byte(""),
				"dsa0/wq0.3/state": []byte(""),
			},
		},
		{
			name:      "invalid: mode entry doesn't exist",
			sysfsDirs: []string{"dsa0/wq0.0", "dsa0/wq0.1", "dsa0/wq0.2", "dsa0/wq0.3"},
			sysfsFiles: map[string][]byte{
				"dsa0/wq0.0/state": []byte("enabled"),
				"dsa0/wq0.1/state": []byte("enabled"),
				"dsa0/wq0.2/state": []byte(""),
				"dsa0/wq0.3/state": []byte(""),
			},
			expectedError: true,
		},
		{
			name:      "invalid: type entry doesn't exist",
			sysfsDirs: []string{"dsa0/wq0.0", "dsa0/wq0.1", "dsa0/wq0.2", "dsa0/wq0.3"},
			sysfsFiles: map[string][]byte{
				"dsa0/wq0.0/state": []byte("enabled"),
				"dsa0/wq0.1/state": []byte("enabled"),
				"dsa0/wq0.2/state": []byte(""),
				"dsa0/wq0.3/state": []byte(""),
				"dsa0/wq0.0/mode":  []byte("dedicated"),
				"dsa0/wq0.1/mode":  []byte("dedicated"),
			},
			expectedError: true,
		},
		{
			name:      "valid: two dedicated user queues",
			sysfsDirs: []string{"dsa0/wq0.0", "dsa0/wq0.1", "dsa0/wq0.2", "dsa0/wq0.3"},
			sysfsFiles: map[string][]byte{
				"dsa0/wq0.0/state": []byte("enabled"),
				"dsa0/wq0.1/state": []byte("enabled"),
				"dsa0/wq0.2/state": []byte(""),
				"dsa0/wq0.3/state": []byte(""),
				"dsa0/wq0.0/mode":  []byte("dedicated"),
				"dsa0/wq0.1/mode":  []byte("dedicated"),
				"dsa0/wq0.0/type":  []byte("user"),
				"dsa0/wq0.1/type":  []byte("user"),
			},
			expectedResult: map[string]int{
				"wq-user-dedicated": 2,
			},
		},
		{
			name:      "valid: two shared user queues x 10",
			sysfsDirs: []string{"dsa0/wq0.0", "dsa0/wq0.1", "dsa0/wq0.2", "dsa0/wq0.3"},
			sysfsFiles: map[string][]byte{
				"dsa0/wq0.0/state": []byte("enabled"),
				"dsa0/wq0.1/state": []byte("enabled"),
				"dsa0/wq0.2/state": []byte(""),
				"dsa0/wq0.3/state": []byte(""),
				"dsa0/wq0.0/mode":  []byte("shared"),
				"dsa0/wq0.1/mode":  []byte("shared"),
				"dsa0/wq0.0/type":  []byte("user"),
				"dsa0/wq0.1/type":  []byte("user"),
			},
			sharedDevNum: 10,
			expectedResult: map[string]int{
				"wq-user-shared": 20,
			},
		},
		{
			name:      "valid: all types of queues",
			sysfsDirs: []string{"dsa0/wq0.0", "dsa0/wq0.1", "dsa1/wq1.0", "dsa1/wq1.1", "dsa1/wq1.2", "dsa1/wq1.3"},
			sysfsFiles: map[string][]byte{
				//device 0
				"dsa0/wq0.0/state": []byte("enabled"),
				"dsa0/wq0.1/state": []byte("enabled"),
				"dsa0/wq0.0/mode":  []byte("shared"),
				"dsa0/wq0.1/mode":  []byte("dedicated"),
				"dsa0/wq0.0/type":  []byte("user"),
				"dsa0/wq0.1/type":  []byte("kernel"),
				//device 1
				"dsa1/wq1.0/state": []byte("enabled"),
				"dsa1/wq1.1/state": []byte("enabled"),
				"dsa1/wq1.2/state": []byte("enabled"),
				"dsa1/wq1.3/state": []byte("enabled"),
				"dsa1/wq1.0/mode":  []byte("shared"),
				"dsa1/wq1.1/mode":  []byte("dedicated"),
				"dsa1/wq1.2/mode":  []byte("shared"),
				"dsa1/wq1.3/mode":  []byte("dedicated"),
				"dsa1/wq1.0/type":  []byte("mdev"),
				"dsa1/wq1.1/type":  []byte("mdev"),
				"dsa1/wq1.2/type":  []byte("mdev"),
				"dsa1/wq1.3/type":  []byte("user"),
			},
			sharedDevNum: 10,
			expectedResult: map[string]int{
				"wq-user-shared":      10,
				"wq-kernel-dedicated": 1,
				"wq-mdev-shared":      20,
				"wq-user-dedicated":   1,
				"wq-mdev-dedicated":   1,
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, genTest(sysfs, statePattern, tc))
	}
}

// generate test to decrease cyclomatic complexity.
func genTest(sysfs, statePattern string, tc testCase) func(t *testing.T) {
	return func(t *testing.T) {
		for _, sysfsdir := range tc.sysfsDirs {
			if err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0750); err != nil {
				t.Fatalf("Failed to create fake sysfs directory: %+v", err)
			}
		}
		for filename, body := range tc.sysfsFiles {
			if err := ioutil.WriteFile(path.Join(sysfs, filename), body, 0600); err != nil {
				t.Fatalf("Failed to create fake sysfs entry: %+v", err)
			}
		}

		plugin := NewDevicePlugin(sysfs, statePattern, "", tc.sharedDevNum)
		plugin.getDevNodes = getFakeDevNodes

		notifier := &fakeNotifier{
			scanDone: plugin.scanDone,
		}

		err := plugin.Scan(notifier)
		if !tc.expectedError && err != nil {
			t.Errorf("unexpected error: %+v", err)
		}
		if tc.expectedError && err == nil {
			t.Errorf("unexpected success")
		}
		if err := checkDeviceTree(notifier.deviceTree, tc.expectedResult, tc.expectedError); err != nil {
			t.Error(err)
		}
	}
}

// checkDeviceTree checks discovered device types and number of discovered devices.
func checkDeviceTree(deviceTree dpapi.DeviceTree, expectedResult map[string]int, expectedError bool) error {
	if !expectedError && deviceTree != nil {
		for key := range deviceTree {
			val, ok := expectedResult[key]
			if !ok {
				return fmt.Errorf("unexpected device type: %s", key)
			}
			numberDev := len(deviceTree[key])
			if numberDev != val {
				return fmt.Errorf("%s: unexpected number of devices: %d, expected: %d", key, numberDev, val)
			}
			delete(expectedResult, key)
		}
		if len(expectedResult) > 0 {
			return fmt.Errorf("missing expected result(s): %+v", expectedResult)
		}
	}
	return nil
}
