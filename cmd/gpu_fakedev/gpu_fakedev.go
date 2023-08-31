// Copyright 2021-2022 Intel Corporation. All Rights Reserved.
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

//---------------------------------------------------------------
// sysfs SPECIFICATION
//
// sys/class/drm/cardX/
// sys/class/drm/cardX/lmem_total_bytes (gpu memory size, number)
// sys/class/drm/cardX/device/
// sys/class/drm/cardX/device/vendor (0x8086)
// sys/class/drm/cardX/device/sriov_numvfs (PF only, number of VF GPUs, number)
// sys/class/drm/cardX/device/drm/
// sys/class/drm/cardX/device/drm/cardX/
// sys/class/drm/cardX/device/drm/renderD1XX/
// sys/class/drm/cardX/device/numa_node (Numa node index[1], number)
// [1] indexing these: /sys/devices/system/node/nodeX/
//---------------------------------------------------------------
// devfs SPECIFICATION
//
// dev/dri/cardX
// dev/dri/renderD1XX
//---------------------------------------------------------------

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/sys/unix"
)

const (
	dirMode    = 0775
	fileMode   = 0644
	cardBase   = 0
	renderBase = 128
	maxDevs    = 128
	sysfsPath  = "sys"
	devfsPath  = "dev"
	mib        = 1024.0 * 1024.0
	// null device major, minor on linux.
	devNullMajor = 1
	devNullMinor = 3
	devNullType  = unix.S_IFCHR
)

var verbose bool

type genOptions struct {
	Capabilities map[string]string // device capabilities mapping for NFD hook
	Info         string            // verbal config description
	DevCount     int               // how many devices to fake
	TilesPerDev  int               // per-device tile count
	DevMemSize   int               // available per-device device-local memory, in bytes
	DevsPerNode  int               // How many devices per Numa node
	VfsPerPf     int               // How many SR-IOV VFs per PF
	// fields for counting what was generated
	files int
	dirs  int
	devs  int
}

func addSysfsDriTree(root string, opts *genOptions, i int) error {
	card := fmt.Sprintf("card%d", cardBase+i)
	base := filepath.Join(root, "class", "drm", card)

	if err := os.MkdirAll(base, dirMode); err != nil {
		return err
	}
	opts.dirs++

	data := []byte(strconv.Itoa(opts.DevMemSize))
	file := filepath.Join(base, "lmem_total_bytes")

	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}
	opts.files++

	path := filepath.Join(base, "device", "drm", card)
	if err := os.MkdirAll(path, dirMode); err != nil {
		return err
	}
	opts.dirs++

	path = filepath.Join(base, "device", "drm", fmt.Sprintf("renderD%d", renderBase+i))
	if err := os.Mkdir(path, dirMode); err != nil {
		return err
	}
	opts.dirs++

	data = []byte("0x8086")
	file = filepath.Join(base, "device", "vendor")

	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}
	opts.files++

	node := 0
	if opts.DevsPerNode > 0 {
		node = i / opts.DevsPerNode
	}

	data = []byte(strconv.Itoa(node))
	file = filepath.Join(base, "device", "numa_node")

	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}
	opts.files++

	if opts.VfsPerPf > 0 && i%(opts.VfsPerPf+1) == 0 {
		data = []byte(strconv.Itoa(opts.VfsPerPf))
		file = filepath.Join(base, "device", "sriov_numvfs")

		if err := os.WriteFile(file, data, fileMode); err != nil {
			return err
		}
		opts.files++
	}

	for tile := 0; tile < opts.TilesPerDev; tile++ {
		path := filepath.Join(base, "gt", fmt.Sprintf("gt%d", tile))
		if err := os.MkdirAll(path, dirMode); err != nil {
			return err
		}
		opts.dirs++
	}

	return nil
}

func addSysfsBusTree(root string, opts *genOptions, i int) error {
	pciName := fmt.Sprintf("0000:00:0%d.0", i)
	base := filepath.Join(root, "bus", "pci", "drivers", "i915", pciName)

	if err := os.MkdirAll(base, dirMode); err != nil {
		return err
	}
	opts.dirs++

	data := []byte("0x4905")
	file := filepath.Join(base, "device")

	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}
	opts.files++

	drm := filepath.Join(base, "drm")
	if err := os.MkdirAll(drm, dirMode); err != nil {
		return err
	}
	opts.dirs++

	return addDeviceNodes(drm, opts, i)
}

func addDeviceNodes(base string, opts *genOptions, i int) error {
	mode := uint32(fileMode | devNullType)
	devid := int(unix.Mkdev(uint32(devNullMajor), uint32(devNullMinor)))

	file := filepath.Join(base, fmt.Sprintf("card%d", cardBase+i))
	if err := unix.Mknod(file, mode, devid); err != nil {
		return fmt.Errorf("NULL device (%d:%d) node creation failed for '%s': %w",
			devNullMajor, devNullMinor, file, err)
	}
	opts.devs++

	file = filepath.Join(base, fmt.Sprintf("renderD%d", renderBase+i))
	if err := unix.Mknod(file, mode, devid); err != nil {
		return fmt.Errorf("NULL device (%d:%d) node creation failed for '%s': %w",
			devNullMajor, devNullMinor, file, err)
	}
	opts.devs++

	return nil
}

func addDevfsDriTree(root string, opts *genOptions, i int) error {
	base := filepath.Join(root, "dri")
	if err := os.MkdirAll(base, dirMode); err != nil {
		return err
	}
	opts.dirs++

	return addDeviceNodes(base, opts, i)
}

func addDebugfsDriTree(root string, opts *genOptions, i int) error {
	base := filepath.Join(root, "kernel", "debug", "dri", strconv.Itoa(i))
	if err := os.MkdirAll(base, dirMode); err != nil {
		return err
	}
	opts.dirs++

	path := filepath.Join(base, "i915_capabilities")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)

	if err != nil {
		return err
	}
	defer f.Close()
	opts.files++

	// keys are in random order which provides extra testing for NFD label parsing code
	for key, value := range opts.Capabilities {
		line := fmt.Sprintf("%s: %s\n", key, value)
		if _, err = f.WriteString(line); err != nil {
			return err
		}
	}

	return nil
}

func removeExistingDir(path, name string) {
	entries, err := os.ReadDir(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Fatalf("ERROR: ReadDir() failed on fake %s path '%s': %v", name, path, err)
	}

	if len(entries) == 0 {
		return
	}

	if name == "sysfs" && len(entries) > 3 {
		log.Fatalf("ERROR: >3 entries in '%s' - real sysfs?", path)
	}

	if name == "devfs" && (entries[0].Name() != "dri" || len(entries) > 1) {
		log.Fatalf("ERROR: >1 entries in '%s', or '%s' != 'dri' - real devfs?", path, entries[0].Name())
	}

	log.Printf("WARN: removing already existing fake %s path '%s'", name, path)

	if err = os.RemoveAll(path); err != nil {
		log.Fatalf("ERROR: removing existing %s in '%s' failed: %v", name, path, err)
	}
}

// generateDriFiles generates the fake sysfs + debugfs + devfs dirs & files according to given options.
func generateDriFiles(opts genOptions) {
	if opts.Info != "" {
		log.Printf("Config: '%s'", opts.Info)
	}

	removeExistingDir(devfsPath, "devfs")
	removeExistingDir(sysfsPath, "sysfs")
	log.Printf("Generating fake DRI device(s) sysfs, debugfs and devfs content under '%s' & '%s'",
		sysfsPath, devfsPath)

	opts.dirs, opts.files = 0, 0
	for i := 0; i < opts.DevCount; i++ {
		if err := addSysfsDriTree(sysfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d sysfs tree generation failed: %v", i, err)
		}

		if err := addDebugfsDriTree(sysfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d debugfs tree generation failed: %v", i, err)
		}

		if err := addDevfsDriTree(devfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d devfs tree generation failed: %v", i, err)
		}

		if err := addSysfsBusTree(sysfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d sysfs bus tree generation failed: %v", i, err)
		}
	}
	log.Printf("Done, created %d dirs, %d devices and %d files.", opts.dirs, opts.devs, opts.files)
}

// getOptions parses options from given JSON file, validates and returns them.
func getOptions(name string) genOptions {
	if name == "" {
		log.Fatal("ERROR: no fake device spec provided")
	}

	data, err := os.ReadFile(name)
	if err != nil {
		log.Fatalf("ERROR: reading JSON spec file '%s' failed: %v", name, err)
	}

	if verbose {
		log.Printf("Using fake device spec: %v\n", string(data))
	}

	var opts genOptions
	if err = json.Unmarshal(data, &opts); err != nil {
		log.Fatalf("ERROR: Unmarshaling JSON spec file '%s' failed: %v", name, err)
	}

	if opts.DevCount < 1 || opts.DevCount > maxDevs {
		log.Fatalf("ERROR: invalid device count: 1 <= %d <= %d", opts.DevCount, maxDevs)
	}

	if opts.VfsPerPf > 0 {
		if opts.TilesPerDev > 0 || opts.DevsPerNode > 0 {
			log.Fatalf("ERROR: SR-IOV VFs (%d) with device tiles (%d) or Numa nodes (%d) is unsupported for faking",
				opts.VfsPerPf, opts.TilesPerDev, opts.DevsPerNode)
		}

		if opts.DevCount%(opts.VfsPerPf+1) != 0 {
			log.Fatalf("ERROR: %d devices cannot be evenly split to between set of 1 SR-IOV PF + %d VFs",
				opts.DevCount, opts.VfsPerPf)
		}
	}

	if opts.DevsPerNode > opts.DevCount {
		log.Fatalf("ERROR: DevsPerNode (%d) > DevCount (%d)", opts.DevsPerNode, opts.DevCount)
	}

	if opts.DevMemSize%mib != 0 {
		log.Fatalf("ERROR: Invalid memory size (%f MiB), not even MiB", float64(opts.DevMemSize)/mib)
	}

	return opts
}

func main() {
	var name string

	flag.StringVar(&name, "json", "", "JSON spec for fake device sysfs, debugfs and devfs content")
	flag.BoolVar(&verbose, "verbose", false, "More verbose output")
	flag.Parse()

	generateDriFiles(getOptions(name))
}
