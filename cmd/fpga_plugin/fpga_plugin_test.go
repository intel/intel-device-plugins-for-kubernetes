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
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/metadata"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

func createTestDirs(devfs, sysfs string, devfsDirs, sysfsDirs []string, sysfsFiles map[string][]byte) error {
	var err error

	for _, devfsdir := range devfsDirs {
		err = os.MkdirAll(path.Join(devfs, devfsdir), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create fake device directory: %+v", err)
		}
	}
	for _, sysfsdir := range sysfsDirs {
		err = os.MkdirAll(path.Join(sysfs, sysfsdir), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create fake device directory: %+v", err)
		}
	}
	for filename, body := range sysfsFiles {
		err = ioutil.WriteFile(path.Join(sysfs, filename), body, 0644)
		if err != nil {
			return fmt.Errorf("Failed to create fake vendor file: %+v", err)
		}
	}

	return nil
}

func TestDiscoverFPGAs(t *testing.T) {
	tmpdir := fmt.Sprintf("/tmp/fpgaplugin-TestDiscoverFPGAs-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sys", "class", "fpga")
	devfs := path.Join(tmpdir, "dev")
	tcases := []struct {
		devfsdirs      []string
		sysfsdirs      []string
		sysfsfiles     map[string][]byte
		expectedResult map[string]map[string]deviceplugin.DeviceInfo
		expectedErr    bool
		mode           pluginMode
	}{
		{
			expectedResult: nil,
			expectedErr:    true,
			mode:           afMode,
		},
		{
			sysfsdirs:      []string{"intel-fpga-dev.0"},
			expectedResult: nil,
			expectedErr:    true,
			mode:           afMode,
		},
		{
			sysfsdirs:      []string{"intel-fpga-dev.0/intel-fpga-port.0"},
			expectedResult: nil,
			expectedErr:    true,
			mode:           afMode,
		},
		{
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			expectedResult: nil,
			expectedErr:    true,
			mode:           afMode,
		},
		{
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
				"intel-fpga-dev.0/intel-fpga-fme.0",
				"intel-fpga-dev.1/intel-fpga-port.1",
				"intel-fpga-dev.1/intel-fpga-fme.1",
				"intel-fpga-dev.2/intel-fpga-port.2",
				"intel-fpga-dev.2/intel-fpga-fme.2",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.1/intel-fpga-port.1/afu_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.2/intel-fpga-port.2/afu_id": []byte("47595d0fae972fbed0c51b4a41c7a349\n"),
			},
			devfsdirs: []string{
				"intel-fpga-port.0", "intel-fpga-fme.0",
				"intel-fpga-port.1", "intel-fpga-fme.1",
				"intel-fpga-port.2", "intel-fpga-fme.2",
			},
			expectedResult: map[string]map[string]deviceplugin.DeviceInfo{
				fmt.Sprintf("%s-d8424dc4a4a3c413f89e433683f9040b", afMode): {
					"intel-fpga-dev.0": {
						State: "Healthy",
						Nodes: []string{
							path.Join(tmpdir, "/dev/intel-fpga-port.0"),
						},
					},
					"intel-fpga-dev.1": {
						State: "Healthy",
						Nodes: []string{
							path.Join(tmpdir, "/dev/intel-fpga-port.1"),
						},
					},
				},
				fmt.Sprintf("%s-47595d0fae972fbed0c51b4a41c7a349", afMode): {
					"intel-fpga-dev.2": {
						State: "Healthy",
						Nodes: []string{
							path.Join(tmpdir, "/dev/intel-fpga-port.2"),
						},
					},
				},
			},
			expectedErr: false,
			mode:        afMode,
		},
		{
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"intel-fpga-dev.0/intel-fpga-fme.1/pr",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.0/intel-fpga-fme.1/pr/interface_id": []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
			},
			expectedResult: nil,
			expectedErr:    true,
			mode:           regionMode,
		},
		{
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
				"intel-fpga-dev.1/intel-fpga-port.1",
			},
			expectedResult: nil,
			expectedErr:    true,
			mode:           regionMode,
		},
		{
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
				"intel-fpga-dev.0/intel-fpga-fme.0/pr",
				"intel-fpga-dev.1/intel-fpga-port.1",
				"intel-fpga-dev.1/intel-fpga-fme.1/pr",
				"intel-fpga-dev.2/intel-fpga-port.2",
				"intel-fpga-dev.2/intel-fpga-fme.2/pr",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id":         []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.1/intel-fpga-port.1/afu_id":         []byte("d8424dc4a4a3c413f89e433683f9040b\n"),
				"intel-fpga-dev.2/intel-fpga-port.2/afu_id":         []byte("47595d0fae972fbed0c51b4a41c7a349\n"),
				"intel-fpga-dev.0/intel-fpga-fme.0/pr/interface_id": []byte("ce48969398f05f33946d560708be108a\n"),
				"intel-fpga-dev.1/intel-fpga-fme.1/pr/interface_id": []byte("ce48969398f05f33946d560708be108a\n"),
				"intel-fpga-dev.2/intel-fpga-fme.2/pr/interface_id": []byte("fd967345645f05f338462a0748be0091\n"),
			},
			devfsdirs: []string{
				"intel-fpga-port.0", "intel-fpga-fme.0",
				"intel-fpga-port.1", "intel-fpga-fme.1",
				"intel-fpga-port.2", "intel-fpga-fme.2",
			},
			expectedResult: map[string]map[string]deviceplugin.DeviceInfo{
				fmt.Sprintf("%s-ce48969398f05f33946d560708be108a", regionMode): {
					"intel-fpga-dev.0": {
						State: "Healthy",
						Nodes: []string{
							path.Join(tmpdir, "/dev/intel-fpga-fme.0"),
							path.Join(tmpdir, "/dev/intel-fpga-port.0"),
						},
					},
					"intel-fpga-dev.1": {
						State: "Healthy",
						Nodes: []string{
							path.Join(tmpdir, "/dev/intel-fpga-fme.1"),
							path.Join(tmpdir, "/dev/intel-fpga-port.1"),
						},
					},
				},
				fmt.Sprintf("%s-fd967345645f05f338462a0748be0091", regionMode): {
					"intel-fpga-dev.2": {
						State: "Healthy",
						Nodes: []string{
							path.Join(tmpdir, "/dev/intel-fpga-fme.2"),
							path.Join(tmpdir, "/dev/intel-fpga-port.2"),
						},
					},
				},
			},
			expectedErr: false,
			mode:        regionMode,
		},
	}

	for _, tcase := range tcases {
		err := createTestDirs(devfs, sysfs, tcase.devfsdirs, tcase.sysfsdirs, tcase.sysfsfiles)
		if err != nil {
			t.Error(err)
		}

		result, err := discoverFPGAs(sysfs, devfs, tcase.mode)
		if tcase.expectedErr && err == nil {
			t.Error("Expected error hasn't been triggered")
		}

		if tcase.expectedResult != nil && !reflect.DeepEqual(result, tcase.expectedResult) {
			t.Logf("Expected result: %+v", tcase.expectedResult)
			t.Logf("Actual result:   %+v", result)
			t.Error("Got unexpeced result")
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatalf("Failed to remove fake device directory: %+v", err)
		}
	}
}

// Minimal implementation of pluginapi.DevicePlugin_ListAndWatchServer
type listAndWatchServerStub struct {
	testDM *deviceManager
	t      *testing.T
	cdata  chan []*pluginapi.Device
	cerr   chan error
}

func (s *listAndWatchServerStub) Send(resp *pluginapi.ListAndWatchResponse) error {
	fmt.Println("listAndWatchServerStub::Send", resp.Devices)
	s.cdata <- resp.Devices
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
	afuID := "d8424dc4a4a3c413f89e433683f9040b"
	tmpdir := fmt.Sprintf("/tmp/fpgaplugin-TestListAndWatch-%d", time.Now().Unix())
	sysfs := path.Join(tmpdir, "sys", "class", "fpga")
	devfs := path.Join(tmpdir, "dev")

	tcases := []struct {
		devfsdirs      []string
		sysfsdirs      []string
		sysfsfiles     map[string][]byte
		expectedResult []*pluginapi.Device
		expectedErr    bool
	}{
		{
			expectedResult: nil,
			expectedErr:    true,
		},
		{
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n"),
			},
			devfsdirs: []string{
				"intel-fpga-port.0", "intel-fpga-fme.0",
			},
			expectedResult: nil,
			expectedErr:    true,
		},
		{
			sysfsdirs: []string{
				"intel-fpga-dev.0/intel-fpga-port.0",
				"intel-fpga-dev.0/intel-fpga-fme.0",
				"intel-fpga-dev.1/intel-fpga-port.1",
				"intel-fpga-dev.1/intel-fpga-fme.1",
				"intel-fpga-dev.2/intel-fpga-port.2",
				"intel-fpga-dev.2/intel-fpga-fme.2",
			},
			sysfsfiles: map[string][]byte{
				"intel-fpga-dev.0/intel-fpga-port.0/afu_id": []byte(afuID),
				"intel-fpga-dev.1/intel-fpga-port.1/afu_id": []byte(afuID),
				"intel-fpga-dev.2/intel-fpga-port.2/afu_id": []byte("47595d0fae972fbed0c51b4a41c7a349\n"),
			},
			devfsdirs: []string{
				"intel-fpga-port.0", "intel-fpga-fme.0",
				"intel-fpga-port.1", "intel-fpga-fme.1",
				"intel-fpga-port.2", "intel-fpga-fme.2",
			},
			expectedResult: []*pluginapi.Device{
				{"intel-fpga-dev.0", "Healthy"},
				{"intel-fpga-dev.1", "Healthy"},
			},
			expectedErr: false,
		},
	}

	resourceName := fmt.Sprintf("%s%s-%s", resourceNamePrefix, afMode, afuID)
	testDM := newDeviceManager(resourceName, fmt.Sprintf("%s-%s", afMode, afuID), tmpdir, afMode)
	if testDM == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	server := &listAndWatchServerStub{
		testDM: testDM,
		t:      t,
		cdata:  make(chan []*pluginapi.Device),
		cerr:   make(chan error),
	}

	for _, tcase := range tcases {
		err := createTestDirs(devfs, sysfs, tcase.devfsdirs, tcase.sysfsdirs, tcase.sysfsfiles)
		if err != nil {
			t.Error(err)
		}

		go func() {
			err = testDM.ListAndWatch(&pluginapi.Empty{}, server)
			if err != nil {
				server.cerr <- err
			}
		}()

		select {
		case result := <-server.cdata:
			if tcase.expectedErr {
				t.Error("Expected error hasn't been triggered")
			} else if tcase.expectedResult != nil && !reflect.DeepEqual(result, tcase.expectedResult) {
				t.Logf("Expected result: %+v", tcase.expectedResult)
				t.Logf("Actual result:   %+v", result)
				t.Error("Got unexpeced result")
			}
			testDM.srv.Stop()
		case err = <-server.cerr:
			if !tcase.expectedErr {
				t.Errorf("Unexpected error has been triggered: %+v", err)
			}
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatalf("Failed to remove fake device directory: %+v", err)
		}
	}
}

func TestAllocate(t *testing.T) {
	testDM := newDeviceManager("", "", "", afMode)
	if testDM == nil {
		t.Fatal("Failed to create a deviceManager")
	}

	rqt := &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{
			{
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

func TestIsValidPluginMode(t *testing.T) {
	tcases := []struct {
		input  pluginMode
		output bool
	}{
		{
			input:  afMode,
			output: true,
		},
		{
			input:  regionMode,
			output: true,
		},
		{
			input:  pluginMode("unparsable"),
			output: false,
		},
	}

	for _, tcase := range tcases {
		if isValidPluginMode(tcase.input) != tcase.output {
			t.Error("Wrong output", tcase.output, "for the given input", tcase.input)
		}
	}
}
