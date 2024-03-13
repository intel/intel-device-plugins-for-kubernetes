// Copyright 2024 Intel Corporation. All Rights Reserved.
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

package pluginutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeviceDriverSymlink(t *testing.T) {
	root, err := os.MkdirTemp("", "test_devicedriver")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	err = os.Mkdir(filepath.Join(root, "i915"), 0777)
	if err != nil {
		t.Errorf("Failed to create required directory structure: %+v", err)
	}

	err = os.Mkdir(filepath.Join(root, "device"), 0777)
	if err != nil {
		t.Errorf("Failed to create required directory structure: %+v", err)
	}

	err = os.Symlink(filepath.Join(root, "i915"), filepath.Join(root, "device", "driver"))
	if err != nil {
		t.Errorf("Failed to create required directory structure: %+v", err)
	}

	driver, err := ReadDeviceDriver(root)

	if err != nil {
		t.Errorf("Got error when there shouldn't be any: %+v", err)
	}

	if driver != "i915" {
		t.Errorf("Got invalid driver: %s", driver)
	}
}

func TestDeviceDriverSymlinkError(t *testing.T) {
	root, err := os.MkdirTemp("", "test_devicedriver")
	if err != nil {
		t.Fatalf("can't create temporary directory: %+v", err)
	}

	defer os.RemoveAll(root)

	err = os.Mkdir(filepath.Join(root, "i915"), 0777)
	if err != nil {
		t.Errorf("Failed to create required directory structure: %+v", err)
	}

	err = os.MkdirAll(filepath.Join(root, "device", "driver"), 0777)
	if err != nil {
		t.Errorf("Failed to create required directory structure: %+v", err)
	}

	_, err = ReadDeviceDriver(root)

	if err == nil {
		t.Errorf("Got no error when there should be one")
	}
}
