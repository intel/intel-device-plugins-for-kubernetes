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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

func init() {
	debug.Activate()
}

func TestScan(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/mdevplugin-test-%d", time.Now().Unix())
	iommuGroupDirectory := path.Join(tmpdir, "sys/kernel/iommu_groups")
	vfioDevicePath := path.Join(tmpdir, "dev/vfio")

	tcases := []struct {
		iommuGroupDirectorydirs []string
		mdevDevicefiles         map[string][]byte
		vfioDevicedirs          []string
		expectedDevs            int
		expectedErr             bool
	}{
		{
			expectedErr:  false,
			expectedDevs: 0,
		},
		{
			vfioDevicedirs: []string{"0"},
			expectedErr:    true,
			expectedDevs:   0,
		},
		{
			iommuGroupDirectorydirs: []string{"0", "0/devices"},
			expectedErr:             false,
			expectedDevs:            0,
		},

		{
			iommuGroupDirectorydirs: []string{"0", "0/devices"},
			mdevDevicefiles: map[string][]byte{
				"0/devices/a297de6a-f4c2-11e6-90f7-cb6a86ce449f": []byte("test uuid"),
			},
			vfioDevicedirs: []string{"0"},
			expectedErr:    false,
			expectedDevs:   1,
		},
		{
			iommuGroupDirectorydirs: []string{"0", "0/devices", "1", "1/devices"},
			mdevDevicefiles: map[string][]byte{
				"0/devices/a297de6a-f4c2-11e6-90f7-cb6a86ce449f": []byte("test uuid"),
				"1/devices/a297db4a-f4c2-11e6-90f6-d3b88d6c9525": []byte("test uuid"),
			},
			vfioDevicedirs: []string{"0", "1"},
			expectedErr:    false,
			expectedDevs:   2,
		},
		{
			iommuGroupDirectorydirs: []string{"0", "0/devices", "2", "2/devices"},
			mdevDevicefiles: map[string][]byte{
				"0/devices/a297de6a-f4c2-11e6-90f7-cb6a86ce449f": []byte("test uuid"),
				"2/devices/a297db4a-f4c2-11e6-90f6-d3b88d6c952":  []byte("test fake uuid"),
			},
			vfioDevicedirs: []string{"0", "2"},
			expectedErr:    false,
			expectedDevs:   1,
		},
	}

	testPlugin := newDevicePlugin(iommuGroupDirectory, vfioDevicePath)

	if testPlugin == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	for _, tcase := range tcases {
		for _, iommuGroupDirectorydir := range tcase.iommuGroupDirectorydirs {
			err := os.MkdirAll(path.Join(iommuGroupDirectory, iommuGroupDirectorydir), 0755)
			if err != nil {
				t.Fatalf("Failed to create iommu group directory: %+v", err)
			}
		}
		for filename, body := range tcase.mdevDevicefiles {
			err := ioutil.WriteFile(path.Join(iommuGroupDirectory, filename), body, 0644)
			if err != nil {
				t.Fatalf("Failed to create mdev device file: %+v", err)
			}
		}

		for _, vfioDevicedir := range tcase.vfioDevicedirs {
			err := os.MkdirAll(path.Join(vfioDevicePath, vfioDevicedir), 0755)
			if err != nil {
				t.Fatalf("Failed to create vfio device directory: %+v", err)
			}
		}

		tree, err := testPlugin.scan()

		if tcase.expectedErr && err == nil {
			t.Error("Expected error hasn't been triggered")
		}
		if !tcase.expectedErr && err != nil {
			t.Errorf("Unexpcted error: %+v", err)
		}
		if tcase.expectedDevs != len(tree[deviceType]) {
			t.Errorf("Wrong number of discovered devices")
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatalf("Failed to remove fake device directory: %+v", err)
		}
	}
}
