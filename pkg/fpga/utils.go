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

package fpga

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// small helper function that reads several files into provided set of variables.
func readFilesInDirectory(fileMap map[string]*string, dir string) error {
	for k, v := range fileMap {
		fname := filepath.Join(dir, k)
		if strings.ContainsAny(fname, "?*[") {
			// path contains wildcards, let's find by Glob needed file.
			files, err := filepath.Glob(fname)
			switch {
			case err != nil:
				continue
			case len(files) != 1:
				// doesn't match unique file, skip it
				// fmt.Println("KAD2: ", files)
				continue
			}
			fname = files[0]
		}
		b, err := ioutil.ReadFile(fname)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return errors.Wrapf(err, "%s: unable to read file %q", dir, k)
		}
		*v = strings.TrimSpace(string(b))
	}
	return nil
}

// returns filename of the argument after resolving symlinks.
func cleanBasename(name string) string {
	realPath, err := filepath.EvalSymlinks(name)
	if err != nil {
		realPath = name
	}
	return filepath.Base(realPath)
}

// check that FPGA device is a compatible PCI device.
func checkVendorAndClass(dev commonFpgaAPI) error {
	pci, err := dev.GetPCIDevice()
	if err != nil {
		return err
	}
	if pci.Vendor != vendorIntel || pci.Class != fpgaClass {
		return errors.Errorf("unsupported PCI device %s  VID=%s PID=%s Class=%s", pci.BDF, pci.Vendor, pci.Device, pci.Class)
	}
	return nil
}
