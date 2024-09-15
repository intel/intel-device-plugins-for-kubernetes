// Copyright 2021-2024 Intel Corporation. All Rights Reserved.
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

package fakedri

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	DirMode    = 0775
	FileMode   = 0644
	CardBase   = 0
	RenderBase = 128
	MaxDevs    = 128
	SysfsPath  = "sys"
	DevfsPath  = "dev"
	Mib        = 1024.0 * 1024.0
	DevNullMajor = 1
	DevNullMinor = 3
	DevNullType  = unix.S_IFCHR
	MaxK8sLabelSize = 63
	FullyConnected  = "FULL"
)

var Verbose bool

type GenOptions struct {
	Capabilities map[string]string
	Info         string
	DevCount     int
	TilesPerDev  int
	DevMemSize   int
	DevsPerNode  int
	VfsPerPf     int
	files        int
	dirs         int
	devs         int
}

func addSysfsDriTree(root string, opts *GenOptions, i int) error {
	card := fmt.Sprintf("card%d", CardBase+i)
	base := filepath.Join(root, "class", "drm", card)

	if err := os.MkdirAll(base, DirMode); err != nil {
		return err
	}
	opts.dirs++

	data := []byte(strconv.Itoa(opts.DevMemSize))
	file := filepath.Join(base, "lmem_total_bytes")

	if err := os.WriteFile(file, data, FileMode); err != nil {
		return err
	}
	opts.files++

	path := filepath.Join(base, "device", "drm", card)
	if err := os.MkdirAll(path, DirMode); err != nil {
		return err
	}
	opts.dirs++

	path = filepath.Join(base, "device", "drm", fmt.Sprintf("renderD%d", RenderBase+i))
	if err := os.Mkdir(path, DirMode); err != nil {
		return err
	}
	opts.dirs++

	data = []byte("0x8086")
	file = filepath.Join(base, "device", "vendor")

	if err := os.WriteFile(file, data, FileMode); err != nil {
		return err
	}
	opts.files++

	node := 0
	if opts.DevsPerNode > 0 {
		node = i / opts.DevsPerNode
	}

	data = []byte(strconv.Itoa(node))
	file = filepath.Join(base, "device", "numa_node")

	if err := os.WriteFile(file, data, FileMode); err != nil {
		return err
	}
	opts.files++

	if opts.VfsPerPf > 0 && i%(opts.VfsPerPf+1) == 0 {
		data = []byte(strconv.Itoa(opts.VfsPerPf))
		file = filepath.Join(base, "device", "sriov_numvfs")

		if err := os.WriteFile(file, data, FileMode); err != nil {
			return err
		}
		opts.files++
	}

	for tile := 0; tile < opts.TilesPerDev; tile++ {
		path := filepath.Join(base, "gt", fmt.Sprintf("gt%d", tile))
		if err := os.MkdirAll(path, DirMode); err != nil {
			return err
		}
		opts.dirs++
	}

	return nil
}

func addSysfsBusTree(root string, opts *GenOptions, i int) error {
	pciName := fmt.Sprintf("0000:00:0%d.0", i)
	base := filepath.Join(root, "bus", "pci", "drivers", "i915", pciName)

	if err := os.MkdirAll(base, DirMode); err != nil {
		return err
	}
	opts.dirs++

	data := []byte("0x4905")
	file := filepath.Join(base, "device")

	if err := os.WriteFile(file, data, FileMode); err != nil {
		return err
	}
	opts.files++

	drm := filepath.Join(base, "drm")
	if err := os.MkdirAll(drm, DirMode); err != nil {
		return err
	}
	opts.dirs++

	return addDeviceNodes(drm, opts, i)
}

func addDeviceNodes(base string, opts *GenOptions, i int) error {
	mode := uint32(FileMode | DevNullType)
	devid := int(unix.Mkdev(uint32(DevNullMajor), uint32(DevNullMinor)))

	file := filepath.Join(base, fmt.Sprintf("card%d", CardBase+i))
	if err := unix.Mknod(file, mode, devid); err != nil {
		return fmt.Errorf("NULL device (%d:%d) node creation failed for '%s': %w",
			DevNullMajor, DevNullMinor, file, err)
	}
	opts.devs++

	file = filepath.Join(base, fmt.Sprintf("renderD%d", RenderBase+i))
	if err := unix.Mknod(file, mode, devid); err != nil {
		return fmt.Errorf("NULL device (%d:%d) node creation failed for '%s': %w",
			DevNullMajor, DevNullMinor, file, err)
	}
	opts.devs++

	return nil
}

func addDevfsDriTree(root string, opts *GenOptions, i int) error {
	base := filepath.Join(root, "dri")
	if err := os.MkdirAll(base, DirMode); err != nil {
		return err
	}
	opts.dirs++

	return addDeviceNodes(base, opts, i)
}

func addDebugfsDriTree(root string, opts *GenOptions, i int) error {
	base := filepath.Join(root, "kernel", "debug", "dri", strconv.Itoa(i))
	if err := os.MkdirAll(base, DirMode); err != nil {
		return err
	}
	opts.dirs++

	path := filepath.Join(base, "i915_capabilities")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, FileMode)

	if err != nil {
		return err
	}
	defer f.Close()
	opts.files++

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

func GenerateDriFiles(opts GenOptions) {
	if opts.Info != "" {
		log.Printf("Config: '%s'", opts.Info)
	}

	removeExistingDir(DevfsPath, "devfs")
	removeExistingDir(SysfsPath, "sysfs")
	log.Printf("Generating fake DRI device(s) sysfs, debugfs and devfs content under '%s' & '%s'",
		SysfsPath, DevfsPath)

	opts.dirs, opts.files = 0, 0
	for i := 0; i < opts.DevCount; i++ {
		if err := addSysfsDriTree(SysfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d sysfs tree generation failed: %v", i, err)
		}

		if err := addDebugfsDriTree(SysfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d debugfs tree generation failed: %v", i, err)
		}

		if err := addDevfsDriTree(DevfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d devfs tree generation failed: %v", i, err)
		}

		if err := addSysfsBusTree(SysfsPath, &opts, i); err != nil {
			log.Fatalf("ERROR: dev-%d sysfs bus tree generation failed: %v", i, err)
		}
	}
	log.Printf("Done, created %d dirs, %d devices and %d files.", opts.dirs, opts.devs, opts.files)

	makeXelinkSideCar(opts)
}

func makeXelinkSideCar(opts GenOptions) {
	topology := opts.Capabilities["connection-topology"]
	gpus := opts.DevCount
	tiles := opts.TilesPerDev
	connections := opts.Capabilities["connections"]

	if topology != FullyConnected {
		saveSideCarFile(connections)
	} else {
		saveSideCarFile(buildConnectionList(gpus, tiles))
	}

	log.Printf("XELINK: generated xelink sidecar label file, using (GPUs: %d, Tiles: %d, Topology: %s)", gpus, tiles, topology)
}

func buildConnectionList(gpus, tiles int) string {
	var nodes = make([]string, 0)

	for mm := 0; mm < gpus; mm++ {
		for nn := 0; nn < tiles; nn++ {
			nodes = append(nodes, fmt.Sprintf("%d.%d", mm, nn))
		}
	}

	var links = make(map[string]bool, 0)

	var smap = make([]string, 0)

	for _, from := range nodes {
		for _, to := range nodes {
			if to == from {
				continue
			}

			link := fmt.Sprintf("%s-%s", to, from)

			reverselink := fmt.Sprintf("%s-%s", from, to)
			if _, exists := links[reverselink]; !exists {
				links[link] = true

				smap = append(smap, link)
			}
		}
	}

	return strings.Join(smap, "_")
}

func saveSideCarFile(connections string) {
	f, err := os.Create("xpum-sidecar-labels.txt")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	line := fmt.Sprintf("xpumanager.intel.com/xe-links=%s", connections[:min(len(connections), MaxK8sLabelSize)])
	fmt.Println(line)

	if _, err := f.WriteString(line + "\n"); err != nil {
		panic(err)
	}

	index := 2

	for i := MaxK8sLabelSize; i < len(connections); i += (MaxK8sLabelSize - 1) {
		line := fmt.Sprintf("xpumanager.intel.com/xe-links%d=Z%s", index, connections[i:min(len(connections), i+MaxK8sLabelSize-1)])
		fmt.Println(line)

		if _, err := f.WriteString(line + "\n"); err != nil {
			panic(err)
		}
		index++
	}
}

func GetOptions(name string) GenOptions {
	if name == "" {
		log.Fatal("ERROR: no fake device spec provided")
	}

	data, err := os.ReadFile(name)
	if err != nil {
		log.Fatalf("ERROR: reading JSON spec file '%s' failed: %v", name, err)
	}

	if Verbose {
		log.Printf("Using fake device spec: %v\n", string(data))
	}

	var opts GenOptions
	if err = json.Unmarshal(data, &opts); err != nil {
		log.Fatalf("ERROR: Unmarshaling JSON spec file '%s' failed: %v", name, err)
	}

	if opts.DevCount < 1 || opts.DevCount > MaxDevs {
		log.Fatalf("ERROR: invalid device count: 1 <= %d <= %d", opts.DevCount, MaxDevs)
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

	if opts.DevMemSize%Mib != 0 {
		log.Fatalf("ERROR: Invalid memory size (%f MiB), not even MiB", float64(opts.DevMemSize)/Mib)
	}

	return opts
}
