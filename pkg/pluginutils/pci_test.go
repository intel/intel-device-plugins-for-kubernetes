// Copyright 2026 Intel Corporation. All Rights Reserved.
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
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePCIDeviceIDs(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "valid single ID",
			input:     "0x1234",
			wantError: false,
		},
		{
			name:      "valid multiple IDs",
			input:     "0x1234,0x5678,0x9abc",
			wantError: false,
		},
		{
			name:      "valid IDs with spaces",
			input:     " 0x1234 , 0x5678 ",
			wantError: false,
		},
		{
			name:      "empty string",
			input:     "",
			wantError: false,
		},
		{
			name:      "invalid ID format",
			input:     "0x1234,abcd",
			wantError: true,
		},
		{
			name:      "invalid hex length",
			input:     "0x123,0x5678",
			wantError: true,
		},
		{
			name:      "extra comma",
			input:     "0x1234,",
			wantError: true,
		},
		{
			name:      "capital hex",
			input:     "0xAA12",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePCIDeviceIDs(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("parsePCIDeviceIDs() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

type TestCaseFilesystem struct {
	symlinkfiles map[string]string
	sysfsfiles   map[string][]byte
	baseSysfs    string
	baseDevfs    string
	sysfsdirs    []string
	devfsdirs    []string
}

func vfioCreateSysfsTestFiles(root string, tc TestCaseFilesystem) (string, string, error) {
	sysfsPath := filepath.Join(root, tc.baseSysfs)
	devfsPath := filepath.Join(root, tc.baseDevfs)

	// Create sysfs dirs
	for _, dir := range tc.sysfsdirs {
		fullPath := filepath.Join(sysfsPath, dir)
		if err := os.MkdirAll(fullPath, 0700); err != nil {
			return "", "", fmt.Errorf("couldn't create test sysfs dir %s: %w", fullPath, err)
		}
	}

	// Create sysfs files
	for file, content := range tc.sysfsfiles {
		fullPath := filepath.Join(sysfsPath, file)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			return "", "", fmt.Errorf("couldn't create test sysfs file dir %s: %w", filepath.Dir(fullPath), err)
		}

		if err := os.WriteFile(fullPath, content, 0o600); err != nil {
			return "", "", fmt.Errorf("couldn't create test sysfs file %s: %w", fullPath, err)
		}
	}

	// Create symlink files
	for link, target := range tc.symlinkfiles {
		fullLinkPath := filepath.Join(sysfsPath, link)
		fullTargetPath := filepath.Join(sysfsPath, target)
		if err := os.MkdirAll(fullTargetPath, 0700); err != nil {
			return "", "", fmt.Errorf("couldn't create test symlink target dir %s: %w", fullTargetPath, err)
		}

		if err := os.Symlink(fullTargetPath, fullLinkPath); err != nil {
			return "", "", fmt.Errorf("couldn't create test symlink %s -> %s: %w", fullLinkPath, fullTargetPath, err)
		}
	}

	// Create devfs dirs
	for _, dir := range tc.devfsdirs {
		fullPath := filepath.Join(devfsPath, dir)
		if err := os.MkdirAll(fullPath, 0700); err != nil {
			return "", "", fmt.Errorf("couldn't create test devfs dir %s: %w", fullPath, err)
		}
	}

	return sysfsPath, devfsPath, nil
}

func TestIsCompatibleGpuVfioDevice(t *testing.T) {
	type TestCaseDetails struct {
		name       string
		allowIds   string
		denyIds    string
		fs         TestCaseFilesystem
		expectPass bool
	}

	tcases := []TestCaseDetails{
		{
			name: "one device",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor": []byte("0x8086"),
					"0000:00:01.0/class":  []byte("0x030000"),
					"0000:00:01.0/device": []byte("0x1234"),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver":      "drivers/vfio-pci",
					"0000:00:01.0/iommu_group": "iommu_groups/4",
				},
			},
			expectPass: true,
		},
		{
			name: "two devices",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/", "0000:00:02.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor": []byte("0x8086"),
					"0000:00:01.0/class":  []byte("0x030000"),
					"0000:00:01.0/device": []byte("0x1234"),
					"0000:00:02.0/vendor": []byte("0x8086"),
					"0000:00:02.0/class":  []byte("0x030000"),
					"0000:00:02.0/device": []byte("0x1234"),
				},
				devfsdirs: []string{
					"vfio/4",
					"vfio/7",
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver":      "drivers/vfio-pci",
					"0000:00:01.0/iommu_group": "iommu_groups/4",
					"0000:00:02.0/driver":      "drivers/vfio-pci",
					"0000:00:02.0/iommu_group": "iommu_groups/7",
				},
			},
			expectPass: true,
		},
		{
			name: "wrong vendor",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor": []byte("0x8088"),
					"0000:00:01.0/class":  []byte("0x030000"),
					"0000:00:01.0/device": []byte("0x1234"),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver":      "drivers/vfio-pci",
					"0000:00:01.0/iommu_group": "iommu_groups/4",
				},
			},
			expectPass: false,
		},
		{
			name: "deny id match",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor": []byte("0x8086"),
					"0000:00:01.0/class":  []byte("0x030000"),
					"0000:00:01.0/device": []byte("0x1234"),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver":      "drivers/vfio-pci",
					"0000:00:01.0/iommu_group": "iommu_groups/4",
				},
			},
			denyIds:    "0x1234",
			expectPass: false,
		},
		{
			name: "allow id non-match",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor": []byte("0x8086"),
					"0000:00:01.0/class":  []byte("0x030000"),
					"0000:00:01.0/device": []byte("0x1235"),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver":      "drivers/vfio-pci",
					"0000:00:01.0/iommu_group": "iommu_groups/4",
				},
			},
			allowIds:   "0x1234",
			expectPass: false,
		},
		{
			name: "wrong class",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor": []byte("0x8086"),
					"0000:00:01.0/class":  []byte("0x040000"),
					"0000:00:01.0/device": []byte("0x1235"),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver":      "drivers/vfio-pci",
					"0000:00:01.0/iommu_group": "iommu_groups/4",
				},
			},
			expectPass: false,
		},
		{
			name: "sriov pf device with vfs",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor":       []byte("0x8086"),
					"0000:00:01.0/class":        []byte("0x030000"),
					"0000:00:01.0/device":       []byte("0x1235"),
					"0000:00:01.0/sriov_numvfs": []byte("2"),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver":      "drivers/vfio-pci",
					"0000:00:01.0/iommu_group": "iommu_groups/4",
				},
			},
			expectPass: false,
		},
		{
			name: "wrong driver",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci/devices",
				baseDevfs: "dev",
				sysfsdirs: []string{"0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"0000:00:01.0/vendor": []byte("0x8086"),
					"0000:00:01.0/class":  []byte("0x030000"),
					"0000:00:01.0/device": []byte("0x1235"),
				},
				symlinkfiles: map[string]string{
					"0000:00:01.0/driver": "drivers/somethingelse",
				},
			},
			expectPass: false,
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp("", "test_pci_device_detection")
			if err != nil {
				t.Fatalf("Can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			sysfs, _, err := vfioCreateSysfsTestFiles(root, tc.fs)
			if err != nil {
				t.Errorf("Unexpected error: %+v", err)
			}

			for _, device := range tc.fs.sysfsdirs {
				dpath := filepath.Join(sysfs, device)
				compatible := IsCompatibleGpuVfioDevice(dpath, tc.allowIds, tc.denyIds)

				if tc.expectPass != compatible {
					t.Errorf("Expected %t, got %t for device %s", tc.expectPass, compatible, dpath)
				}
			}
		})
	}
}

func TestBindDeviceToDriver(t *testing.T) {
	type TestCaseDetails struct {
		expectContents map[string]string
		name           string
		fs             TestCaseFilesystem
		expectPass     bool
	}

	tcases := []TestCaseDetails{
		{
			name: "already bound to vfio-pci",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci",
				baseDevfs: "dev",
				sysfsdirs: []string{"devices/0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"devices/0000:00:01.0/vendor": []byte("0x8086"),
					"devices/0000:00:01.0/class":  []byte("0x030000"),
					"devices/0000:00:01.0/device": []byte("0x1234"),
					"drivers/xe/new_id":           []byte(""),
					"drivers/xe/unbind":           []byte(""),
					"drivers/vfio-pci/new_id":     []byte(""),
					"drivers/vfio-pci/unbind":     []byte(""),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"devices/0000:00:01.0/driver": "drivers/vfio-pci",
				},
			},
			expectPass: true,
		},
		{
			name: "bind from xe to vfio-pci",
			fs: TestCaseFilesystem{
				baseSysfs: "sys/bus/pci",
				baseDevfs: "dev",
				sysfsdirs: []string{"devices/0000:00:01.0/"},
				sysfsfiles: map[string][]byte{
					"devices/0000:00:01.0/vendor": []byte("0x8086"),
					"devices/0000:00:01.0/class":  []byte("0x030000"),
					"devices/0000:00:01.0/device": []byte("0x1234"),
					"drivers/xe/new_id":           []byte(""),
					"drivers/xe/unbind":           []byte(""),
					"drivers/vfio-pci/new_id":     []byte(""),
					"drivers/vfio-pci/bind":       []byte(""),
				},
				devfsdirs: []string{
					"vfio/4",
				},
				symlinkfiles: map[string]string{
					"devices/0000:00:01.0/driver": "drivers/xe",
				},
			},
			expectPass: false,
			expectContents: map[string]string{
				"drivers/vfio-pci/new_id": "8086 1234",
				"drivers/vfio-pci/bind":   "0000:00:01.0",
				"drivers/xe/unbind":       "0000:00:01.0",
			},
		},
	}

	for _, tc := range tcases {
		t.Run(tc.name, func(t *testing.T) {
			root, err := os.MkdirTemp("", "test_pci_bind_to_driver")
			if err != nil {
				t.Fatalf("Can't create temporary directory: %+v", err)
			}
			// dirs/files need to be removed for the next test
			defer os.RemoveAll(root)

			sysfs, _, err := vfioCreateSysfsTestFiles(root, tc.fs)
			if err != nil {
				t.Errorf("Unexpected error: %+v", err)
			}

			driversPath := filepath.Join(sysfs, "drivers")

			for _, device := range tc.fs.sysfsdirs {
				dpath := filepath.Join(sysfs, device)
				bindOk := BindDeviceToDriver(dpath, driversPath, "vfio-pci")

				if tc.expectPass && bindOk != nil {
					t.Errorf("Expected %t, got %t for device %s", tc.expectPass, bindOk, dpath)
				}
				if tc.expectContents != nil {
					for fakefile, expectedContent := range tc.expectContents {
						fullpath := filepath.Join(sysfs, fakefile)

						content, err := os.ReadFile(fullpath)
						if err != nil {
							t.Errorf("Couldn't read file %s: %+v", fullpath, err)
						}
						if string(content) != expectedContent {
							t.Errorf("Expected file %s to have content '%s', got '%s'", fullpath, expectedContent, string(content))
						}
					}
				}
			}
		})
	}
}
