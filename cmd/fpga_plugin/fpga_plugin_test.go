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

package main

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/pkg/errors"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
)

func init() {
	debug.Activate()
}

func createTestDirs(devfs, sysfs string, devfsDirs, sysfsDirs []string, sysfsFiles map[string][]byte) error {
	var err error

	for _, devfsdir := range devfsDirs {
		err = os.MkdirAll(path.Join(devfs, devfsdir), 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}
	for _, sysfsdir := range sysfsDirs {
		err = os.MkdirAll(path.Join(sysfs, sysfsdir), 0755)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake device directory")
		}
	}
	for filename, body := range sysfsFiles {
		err = ioutil.WriteFile(path.Join(sysfs, filename), body, 0644)
		if err != nil {
			return errors.Wrap(err, "Failed to create fake vendor file")
		}
	}

	return nil
}

func TestPostAllocate(t *testing.T) {
	response := new(pluginapi.AllocateResponse)
	cresp := new(pluginapi.ContainerAllocateResponse)
	response.ContainerResponses = append(response.ContainerResponses, cresp)

	testValue := "some value"

	dp := &devicePlugin{
		annotationValue: testValue,
	}
	dp.PostAllocate(response)

	if len(response.ContainerResponses[0].Annotations) != 1 {
		t.Fatal("Set wrong number of annotations")
	}

	annotation, ok := response.ContainerResponses[0].Annotations[annotationName]
	if ok == false {
		t.Fatalf("%s annotation is not set", annotationName)
	}

	if annotation != testValue {
		t.Fatalf("Set wrong annotation %s", annotation)
	}
}
