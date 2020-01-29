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

package main

import (
	"github.com/google/gousb"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
	"os"
	"testing"
)

func init() {
	debug.Activate()
}

type testCase struct {
	vendorID   int
	productIDs []int
	sharedNum  int
}

//try to inject gousb compatible fake device info
func (t *testCase) OpenDevices(opener func(desc *gousb.DeviceDesc) bool) ([]*gousb.Device, error) {
	var ret []*gousb.Device
	for _, p := range t.productIDs {
		desc := &gousb.DeviceDesc{
			Vendor:  gousb.ID(t.vendorID),
			Product: gousb.ID(p),
		}
		if opener(desc) {
			// only fake desc is enough
			ret = append(ret, &gousb.Device{Desc: desc})
		}
	}
	return ret, nil
}

func TestScan(t *testing.T) {
	f, err := os.Create("/var/tmp/hddl_service.sock")
	defer f.Close()
	if err != nil {
		t.Error("create fake hddl file failed")
	}
	//inject our fake gousbContext, just borrow vendorID and productIDs from main
	tc := &testCase{
		vendorID: vendorID,
	}
	//inject some productIDs that not match our target too
	tc.productIDs = append(productIDs, 0xdead, 0xbeef)
	testPlugin := newDevicePlugin(tc, vendorID, productIDs, 10)

	if testPlugin == nil {
		t.Error("vpu plugin test failed")
	}

	tree, err := testPlugin.scan()
	if err != nil {
		t.Error("vpu plugin test failed")
	} else {
		debug.Printf("tree len is %d", len(tree[deviceType]))
	}
}
