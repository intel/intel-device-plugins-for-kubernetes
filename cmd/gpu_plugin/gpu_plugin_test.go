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
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func init() {
	flag.Set("v", "4") //Enable debug output
}

func TestScan(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/gpuplugin-test-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sysfs")
	devfs := path.Join(tmpdir, "devfs")
	tcases := []struct {
		devfsdirs    []string
		sysfsdirs    []string
		sysfsfiles   map[string][]byte
		expectedDevs int
		expectedErr  bool
	}{
		{
			expectedErr:  true,
			expectedDevs: 0,
		},
		{
			sysfsdirs:    []string{"card0"},
			expectedDevs: 0,
			expectedErr:  false,
		},
		{
			sysfsdirs: []string{"card0/device"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
			expectedDevs: 0,
			expectedErr:  true,
		},
		{
			sysfsdirs: []string{"card0/device/drm/card0"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
			},
			devfsdirs:    []string{"card0"},
			expectedDevs: 1,
			expectedErr:  false,
		},
		{
			sysfsdirs: []string{
				"card0/device/drm/card0",
				"card1/device/drm/card1",
			},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0x8086"),
				"card1/device/vendor": []byte("0x8086"),
			},
			devfsdirs:    []string{"card0"},
			expectedDevs: 1,
			expectedErr:  false,
		},
		{
			sysfsdirs: []string{"card0/device/drm/card0"},
			sysfsfiles: map[string][]byte{
				"card0/device/vendor": []byte("0xbeef"),
			},
			devfsdirs:    []string{"card0"},
			expectedDevs: 0,
			expectedErr:  false,
		},
		{
			sysfsdirs:    []string{"non_gpu_card"},
			expectedDevs: 0,
			expectedErr:  false,
		},
	}

	testPlugin := newDevicePlugin(sysfs, devfs, 1)

	if testPlugin == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	for _, tcase := range tcases {
		for _, devfsdir := range tcase.devfsdirs {
			err := os.MkdirAll(path.Join(devfs, devfsdir), 0755)
			if err != nil {
				t.Fatalf("Failed to create fake device directory: %+v", err)
			}
		}
		for _, sysfsdir := range tcase.sysfsdirs {
			err := os.MkdirAll(path.Join(sysfs, sysfsdir), 0755)
			if err != nil {
				t.Fatalf("Failed to create fake device directory: %+v", err)
			}
		}
		for filename, body := range tcase.sysfsfiles {
			err := ioutil.WriteFile(path.Join(sysfs, filename), body, 0644)
			if err != nil {
				t.Fatalf("Failed to create fake vendor file: %+v", err)
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
