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

package kerneldrv

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
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
 qat_dev0 - type: c6xx,  inst_id: 0,  node_id: 0,  bsf: 0000:3b:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev1 - type: c6xx,  inst_id: 1,  node_id: 0,  bsf: 0000:3d:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev2 - type: c6xx,  inst_id: 2,  node_id: 3,  bsf: 0000:d8:00.0,  #accel: 5 #engines: 10 state: up
`
	adfCtlOutputOneDown = `Checking status of all devices.
There is 3 QAT acceleration device(s) in the system:
 qat_dev0 - type: c6xx,  inst_id: 0,  node_id: 0,  bsf: 3b:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev1 - type: c6xx,  inst_id: 1,  node_id: 0,  bsf: 3d:00.0,  #accel: 5 #engines: 10 state: down
 qat_dev2 - type: c6xx,  inst_id: 2,  node_id: 3,  bsf: d8:00.0,  #accel: 5 #engines: 10 state: up
`
	adfCtlOutputVf = `Checking status of all devices.
There is 7 QAT acceleration device(s) in the system:
 qat_dev0 - type: c6xx,  inst_id: 0,  node_id: 0,  bsf: 0000:3b:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev1 - type: c6xx,  inst_id: 1,  node_id: 0,  bsf: 0000:3b:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev2 - type: c6xx,  inst_id: 2,  node_id: 3,  bsf: 0000:3b:00.0,  #accel: 5 #engines: 10 state: up
 qat_dev3 - type: c6xxvf,  inst_id: 0,  node_id: 0,  bsf: 0000:3b:01.0,  #accel: 1 #engines: 1 state: up
 qat_dev4 - type: c6xxvf,  inst_id: 1,  node_id: 0,  bsf: 0000:3b:01.1,  #accel: 1 #engines: 1 state: up
 qat_dev5 - type: c6xxvf,  inst_id: 2,  node_id: 0,  bsf: 0000:3b:01.2,  #accel: 1 #engines: 1 state: up
 qat_dev6 - type: c6xxvf,  inst_id: 3,  node_id: 0,  bsf: 0000:3b:01.3,  #accel: 1 #engines: 1 state: up
`
)

func init() {
	debug.Activate()
}

func TestGetOnlineDevices(t *testing.T) {
	tcases := []struct {
		name           string
		adfCtlOutput   string
		adfCtlError    error
		expectedDevNum int
		expectedErr    bool
		iommuOn        bool
	}{
		{
			name:           "all is good",
			adfCtlOutput:   adfCtlOutput,
			expectedDevNum: 3,
			iommuOn:        false,
		},
		{
			name:           "one device is down",
			adfCtlOutput:   adfCtlOutputOneDown,
			expectedDevNum: 2,
			iommuOn:        false,
		},
		{
			name:           "virtual functions enabled",
			adfCtlOutput:   adfCtlOutputVf,
			expectedDevNum: 4,
			iommuOn:        true,
		},
		{
			name:        "adf_ctl fails to run",
			adfCtlError: errors.New("fake error"),
			expectedErr: true,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			fcmd := fakeexec.FakeCmd{
				CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
					func() ([]byte, error) {
						return []byte(tt.adfCtlOutput), tt.adfCtlError
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
			dp := &DevicePlugin{
				execer: &execer,
			}
			devices, err := dp.getOnlineDevices(tt.iommuOn)
			if tt.expectedErr && err == nil {
				t.Error("Expected error hasn't been triggered")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Unexpected error: %+v", err)
			}
			if len(devices) != tt.expectedDevNum {
				t.Errorf("Wrong number of device detected: %d instead of %d", len(devices), tt.expectedDevNum)
			}
		})
	}
}

func TestGetUIODevices(t *testing.T) {
	tcases := []struct {
		name        string
		devType     string
		bsf         string
		expectedErr bool
		uiodevs     []string
	}{
		{
			name:        "can't read sysfs",
			devType:     "faketype",
			expectedErr: true,
		},
		{
			name:    "all is good",
			devType: "c6xx",
			uiodevs: []string{"uio0", "uio1"},
			bsf:     "da:00.0",
		},
	}
	for tnum, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			tmpdir := fmt.Sprintf("/tmp/qatplugin-getUIODevices-%d-%d", time.Now().Unix(), tnum)
			sysfs := filepath.Join(tmpdir, "sys")

			for _, uiodev := range tt.uiodevs {
				err = os.MkdirAll(filepath.Join(getUIODeviceListPath(sysfs, tt.devType, tt.bsf), uiodev), 0755)
				if err != nil {
					t.Fatal(err)
				}
			}
			devs, err := getUIODevices(sysfs, tt.devType, tt.bsf)
			if tt.expectedErr && err == nil {
				t.Error("Expected error hasn't been triggered")
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Unexpected error: %+v", err)
			}
			sort.Strings(tt.uiodevs)
			sort.Strings(devs)
			if tt.uiodevs != nil && !reflect.DeepEqual(devs, tt.uiodevs) {
				t.Error("Unexpected devices: ", devs)
			}

			err = os.RemoveAll(tmpdir)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestParseConfigs(t *testing.T) {
	qatdevs := []device{
		{
			id:      "dev0",
			devtype: "c6xx",
		},
		{
			id:      "dev1",
			devtype: "c6xx",
		},
		{
			id:      "dev2",
			devtype: "c6xx",
		},
	}
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
		{
			name:        "Can't parse NumProcesses",
			testData:    "cant_parse_num_processes",
			expectedErr: true,
		},
		{
			name:        "Inconsistent LimitDevAccess",
			testData:    "inconsistent_limitdev",
			expectedErr: true,
		},
	}
	for _, tt := range tcases {
		dp := &DevicePlugin{
			configDir: "./test_data/" + tt.testData,
		}
		_, err := dp.parseConfigs(qatdevs)
		if tt.expectedErr && err == nil {
			t.Errorf("Test case '%s': expected error hasn't been triggered", tt.name)
		}
		if !tt.expectedErr && err != nil {
			t.Errorf("Test case '%s': Unexpected error: %+v", tt.name, err)
		}
	}
}

func TestGetDevTree(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/qatplugin-getDevTree-%d", time.Now().Unix())
	tcases := []struct {
		name        string
		sysfs       string
		uiodevs     map[string][]string
		qatdevs     []device
		config      map[string]section
		expectedErr bool
	}{
		{
			name:  "All is good",
			sysfs: "sys",
			uiodevs: map[string][]string{
				"da:00.0": {"uio4", "uio5"},
			},
			qatdevs: []device{
				{
					id:      "dev0",
					devtype: "c6xx",
					bsf:     "da:00.0",
				},
			},
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
			name:  "Wrong devfs",
			sysfs: "wrongdev",
			qatdevs: []device{
				{
					id:      "dev0",
					devtype: "c6xx",
					bsf:     "da:00.0",
				},
			},
			expectedErr: true,
		},
	}
	for _, tt := range tcases {
		t.Run(tt.name, func(t *testing.T) {
			var err error

			sysfs := filepath.Join(tmpdir, "sys")
			err = os.MkdirAll(sysfs, 0755)
			if err != nil {
				t.Fatal(err)
			}

			for _, qatdev := range tt.qatdevs {
				for _, uiodev := range tt.uiodevs[qatdev.bsf] {
					err = os.MkdirAll(filepath.Join(getUIODeviceListPath(sysfs, qatdev.devtype, qatdev.bsf), uiodev), 0755)
					if err != nil {
						t.Fatal(err)
					}

				}
			}

			_, err = getDevTree(path.Join(tmpdir, tt.sysfs), tt.qatdevs, tt.config)
			if tt.expectedErr && err == nil {
				t.Errorf("Test case '%s': expected error hasn't been triggered", tt.name)
			}
			if !tt.expectedErr && err != nil {
				t.Errorf("Test case '%s': Unexpected error: %+v", tt.name, err)
			}

			err = os.RemoveAll(tmpdir)
			if err != nil {
				t.Fatal(err)
			}
		})
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

		dp := &DevicePlugin{}

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
