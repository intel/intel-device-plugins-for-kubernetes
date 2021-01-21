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
	"flag"
	"os"
	"testing"

	"github.com/google/gousb"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"k8s.io/klog/v2"
)

func init() {
	_ = flag.Set("v", "4")
}

type testCase struct {
	vendorID   int
	productIDs []int
	sharedNum  int
}

//OpenDevices tries to inject gousb compatible fake device info.
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

// fakeNotifier implements Notifier interface.
type fakeNotifier struct {
	scanDone chan bool
	tree     dpapi.DeviceTree
}

// Notify stops plugin Scan.
func (n *fakeNotifier) Notify(newDeviceTree dpapi.DeviceTree) {
	n.tree = newDeviceTree
	n.scanDone <- true
}

func TestScan(t *testing.T) {
	var fN fakeNotifier

	f, err := os.Create(hddlSockPath)
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
		t.Fatal("vpu plugin test failed with newDevicePlugin().")
	}

	fN.scanDone = testPlugin.scanDone
	err = testPlugin.Scan(&fN)
	if err != nil {
		t.Error("vpu plugin test failed with testPlugin.Scan()")
	}
	if len(fN.tree[deviceType]) == 0 {
		t.Error("vpu plugin test failed with testPlugin.Scan(): tree len is 0")
	}
	klog.V(4).Infof("tree len is %d", len(fN.tree[deviceType]))

	//remove the hddl_service.sock and test with no hddl socket case
	_ = f.Close()
	_ = os.Remove("/var/tmp/hddl_service.sock")
	testPlugin = newDevicePlugin(tc, vendorID, productIDs, 10)

	if testPlugin == nil {
		t.Fatal("vpu plugin test failed with newDevicePlugin() in no hddl_service.sock case.")
	}

	fN.scanDone = testPlugin.scanDone
	err = testPlugin.Scan(&fN)
	if err != nil {
		t.Error("vpu plugin test failed with testPlugin.Scan() in no hddl_service.sock case.")
	}
	if len(fN.tree[deviceType]) != 0 {
		t.Error("vpu plugin test failed with testPlugin.Scan(): tree len should be 0 in no hddl_service.sock case.")
	}

	//test with sharedNum equals 0 case
	testPlugin = newDevicePlugin(tc, vendorID, productIDs, tc.sharedNum)
	if testPlugin != nil {
		t.Error("vpu plugin test fail: newDevicePlugin should fail with 0 sharedDevNum")
	}
}
