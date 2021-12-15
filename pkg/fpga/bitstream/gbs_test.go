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
	"path/filepath"
	"testing"
)

func TestOpenGBS(t *testing.T) {
	tcases := []struct {
		name          string
		fname         string
		expectedError bool
	}{
		{
			name:  "correct GBS",
			fname: "testdata/intel.com/fpga/69528db6eb31577a8c3668f9faa081f6/d8424dc4a4a3c413f89e433683f9040b.gbs",
		},
		{
			name:          "file doesn't exist",
			fname:         "itdoesntexist.gbs",
			expectedError: true,
		},
	}
	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := OpenGBS(tc.fname)
			if tc.expectedError && err == nil {
				t.Error("unexpected success")
			}
			if !tc.expectedError && err != nil {
				t.Errorf("unexpected error: %+v", err)
			}
		})
	}
}

func TestFileGBSMethods(t *testing.T) {
	interfaceUUID := "69528db6eb31577a8c3668f9faa081f6"
	typeUUID := "d8424dc4a4a3c413f89e433683f9040b"

	gbs, err := OpenGBS(filepath.Join("testdata/intel.com/fpga", interfaceUUID, typeUUID) + ".gbs")
	if err != nil {
		t.Errorf("unexpected open error: %+v", err)
		return
	}

	reader := gbs.RawBitstreamReader()
	if reader == nil {
		t.Error("unexpected nil bitstream reader")
		return
	}

	_, err = gbs.RawBitstreamData()
	if err != nil {
		t.Errorf("unexpected data error: %+v", err)
		return
	}

	intUUID := gbs.InterfaceUUID()
	if intUUID != interfaceUUID {
		t.Errorf("unexpected Interface UUID value: %s", intUUID)
	}

	installPath := gbs.InstallPath("")
	if installPath != filepath.Join(interfaceUUID, typeUUID)+".gbs" {
		t.Errorf("unexpected Install Path value: %s", installPath)
	}

	typUUID := gbs.AcceleratorTypeUUID()
	if typUUID != typeUUID {
		t.Errorf("unexpected Accelerator type UUID value: %s", typUUID)
	}

	extraMD := gbs.ExtraMetadata()
	if extraMD == nil || extraMD["Size"] != "1" {
		t.Errorf("unexpected extra metadata: %+v", extraMD)
	}

	err = gbs.Close()
	if err != nil {
		t.Errorf("unexpected close error: %+v", err)
	}
}
