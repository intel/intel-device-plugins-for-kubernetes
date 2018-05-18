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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

// Minimal implementation of pluginapi.DevicePlugin_ListAndWatchServer
type listAndWatchServerStub struct {
	counter int
	testDM  *deviceManager
	t       *testing.T
}

func (s *listAndWatchServerStub) Send(resp *pluginapi.ListAndWatchResponse) error {
	if s.counter > 0 {
		return errors.New("Fake error when sending response")
	}

	if len(resp.Devices) != 1 {
		s.t.Error("Wrong number of sent device infos")
	}

	s.testDM.devices["dev1"] = deviceplugin.DeviceInfo{pluginapi.Unhealthy, []string{"/dev/dev1"}}
	s.counter++
	return nil
}

func (s *listAndWatchServerStub) Context() context.Context {
	return nil
}

func (s *listAndWatchServerStub) RecvMsg(m interface{}) error {
	return nil
}

func (s *listAndWatchServerStub) SendMsg(m interface{}) error {
	return nil
}

func (s *listAndWatchServerStub) SendHeader(m metadata.MD) error {
	return nil
}

func (s *listAndWatchServerStub) SetHeader(m metadata.MD) error {
	return nil
}

func (s *listAndWatchServerStub) SetTrailer(m metadata.MD) {
}

func TestListAndWatch(t *testing.T) {
	testDM := newDeviceManager()

	if testDM == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	testDM.devices["dev1"] = deviceplugin.DeviceInfo{pluginapi.Healthy, []string{"/dev/dev1"}}

	server := &listAndWatchServerStub{
		testDM: testDM,
		t:      t,
	}

	testDM.ListAndWatch(&pluginapi.Empty{}, server)
}

func TestAllocate(t *testing.T) {
	testDM := newDeviceManager()

	if testDM == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	rqt := &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			&pluginapi.ContainerAllocateRequest{
				DevicesIDs: []string{"dev1"},
			},
		},
	}

	testDM.devices["dev1"] = deviceplugin.DeviceInfo{pluginapi.Healthy, []string{"/dev/dev1"}}
	resp, err := testDM.Allocate(nil, rqt)
	if err != nil {
		t.Fatalf("Failed to allocate healthy device: %+v", err)
	}

	if len(resp.ContainerResponses[0].Devices) != 1 {
		t.Fatal("Allocated wrong number of devices")
	}
}

func TestDiscoverGPUs(t *testing.T) {
	var err error

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
	}

	testDM := newDeviceManager()

	if testDM == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	for _, tcase := range tcases {
		for _, devfsdir := range tcase.devfsdirs {
			err = os.MkdirAll(path.Join(devfs, devfsdir), 0755)
			if err != nil {
				t.Fatalf("Failed to create fake device directory: %+v", err)
			}
		}
		for _, sysfsdir := range tcase.sysfsdirs {
			err = os.MkdirAll(path.Join(sysfs, sysfsdir), 0755)
			if err != nil {
				t.Fatalf("Failed to create fake device directory: %+v", err)
			}
		}
		for filename, body := range tcase.sysfsfiles {
			err = ioutil.WriteFile(path.Join(sysfs, filename), body, 0644)
			if err != nil {
				t.Fatalf("Failed to create fake vendor file: %+v", err)
			}
		}

		err = testDM.discoverGPUs(sysfs, devfs)
		if tcase.expectedErr && err == nil {
			t.Error("Expected error hasn't been triggered")
		}
		if tcase.expectedDevs != len(testDM.devices) {
			t.Errorf("Wrong number of discovered devices")
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatalf("Failed to remove fake device directory: %+v", err)
		}
	}
}
