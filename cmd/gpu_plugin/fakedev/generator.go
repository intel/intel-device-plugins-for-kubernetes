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
	"flag"
	"fmt"
	"log"
	"os"

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
	// null device major, minor on linux
	devNullMajor = 1
	devNullMinor = 3
	devNullType  = unix.S_IFCHR
)

var verbose bool

type genOptions struct {
	Info         string            // verbal config description
	DevCount     int               // how many devices to fake
	TilesPerDev  int               // per-device tile count
	DevMemSize   int               // available per-device device-local memory, in bytes
	DevsPerNode  int               // How many devices per Numa node
	VfsPerPf     int               // How many SR-IOV VFs per PF
	Capabilities map[string]string // device capabilities mapping for NFD hook
	// fields for counting what was generated
	files int
	dirs  int
	devs  int
}

func addSysfsDriTree(root string, opts *genOptions, i int) error {
	card := cardBase + i
	base := fmt.Sprintf("%s/class/drm/card%d", root, card)
	if err := os.MkdirAll(base, dirMode); err != nil {
		return err
	}
	opts.dirs++

	data := []byte(fmt.Sprintf("%d", opts.DevMemSize))
	file := fmt.Sprintf("%s/lmem_total_bytes", base)
	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}
	opts.files++

	path := fmt.Sprintf("%s/device/drm/card%d", base, card)
	if err := os.MkdirAll(path, dirMode); err != nil {
		return err
	}
	opts.dirs++

	path = fmt.Sprintf("%s/device/drm/renderD%d", base, renderBase+i)
	if err := os.Mkdir(path, dirMode); err != nil {
		return err
	}
	opts.dirs++

	data = []byte("0x8086")
	file = fmt.Sprintf("%s/device/vendor", base)
	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}
	opts.files++

	node := 0
	if opts.DevsPerNode > 0 {
		node = i / opts.DevsPerNode
	}
	data = []byte(fmt.Sprintf("%d", node))
	file = fmt.Sprintf("%s/device/numa_node", base)
	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}
	opts.files++

	if opts.VfsPerPf > 0 && i%(opts.VfsPerPf+1) == 0 {
		data = []byte(fmt.Sprintf("%d", opts.VfsPerPf))
		file = fmt.Sprintf("%s/device/sriov_numvfs", base)
		if err := os.WriteFile(file, data, fileMode); err != nil {
			return err
		}
		opts.files++
	}
	for tile := 0; tile < opts.TilesPerDev; tile++ {
		path := fmt.Sprintf("%s/gt/gt%d", base, tile)
		if err := os.MkdirAll(path, dirMode); err != nil {
			return err
		}
		opts.dirs++
	}
	return nil
}

func addDevfsDriTree(root string, opts *genOptions, i int) error {
	base := fmt.Sprintf("%s/dri", root)
	if err := os.MkdirAll(base, dirMode); err != nil {
		return err
	}
	opts.dirs++

	mode := uint32(fileMode | devNullType)
	devid := int(unix.Mkdev(uint32(devNullMajor), uint32(devNullMinor)))

	file := fmt.Sprintf("%s/card%d", base, cardBase+i)
	if err := unix.Mknod(file, mode, devid); err != nil {
		return err
	}
	opts.devs++

	file = fmt.Sprintf("%s/renderD%d", base, renderBase+i)
	if err := unix.Mknod(file, mode, devid); err != nil {
		return err
	}
	opts.devs++
	return nil
}

func addDebugfsDriTree(root string, opts *genOptions, i int) error {
	base := fmt.Sprintf("%s/kernel/debug/dri/%d", root, i)
	os.MkdirAll(base, dirMode)
	opts.dirs++

	path := fmt.Sprintf("%s/i915_capabilities", base)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return err
	}
	opts.files++
	defer f.Close()

	// keys are in random order which provides extra testing for NFD label parsing code
	for key, value := range opts.Capabilities {
		line := fmt.Sprintf("%s: %s\n", key, value)
		if _, err = f.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

// generateDriFiles generarates the fake sysfs + debugfs + devfs dirs & files according to given options
func generateDriFiles(opts genOptions) {
	if opts.Info != "" {
		log.Printf("Config: '%s'", opts.Info)
	}
	entries, _ := os.ReadDir(devfsPath)
	if len(entries) > 0 {
		if len(entries) > 1 || entries[0].Name() != "dri" {
			log.Fatalf("ERROR: >1 entries in '%s', or '%s' != 'dri' - real devfs?", devfsPath, entries[0].Name())
		}
		log.Printf("WARN: removing already existing %s'", devfsPath)
		os.RemoveAll(devfsPath)
	}
	entries, _ = os.ReadDir(sysfsPath)
	if len(entries) > 0 {
		log.Printf("WARN: removing already existing '%s'", sysfsPath)
		os.RemoveAll(sysfsPath)
	}
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
	}
	log.Printf("Done, created %d dirs, %d devices and %d files.", opts.dirs, opts.devs, opts.files)
}

// getOptions parses options from given JSON file, validates and returns them
func getOptions(name string) genOptions {
	if name == "" {
		log.Fatal("ERROR: no fake device spec provided")
	}
	var err error
	var data []byte
	if data, err = os.ReadFile(name); err != nil {
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
