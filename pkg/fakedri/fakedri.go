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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"gopkg.in/yaml.v2"

	"k8s.io/klog/v2"
)

const (
	dirMode         = 0775
	fileMode        = 0644
	cardBase        = 0
	renderBase      = 128
	maxDevs         = 128
	sysfsPath       = "/sys"
	devfsPath       = "/dev"
	mib             = 1024.0 * 1024.0
	devNullMajor    = 1
	devNullMinor    = 3
	devNullType     = unix.S_IFCHR
	maxK8sLabelSize = 63
	fullyConnected  = "FULL"
)

// genOptionsWithTags represents the struct for our YAML data.
type GenOptions struct {
	Capabilities    map[string]string `yaml:"Capabilities"`    // Device capabilities mapping for NFD hook
	Info            string            `yaml:"Info"`            // Verbal config description
	Driver          string            `yaml:"Driver"`          // Driver name (i915, xe)
	Mode            string            `yaml:"Mode"`            // Mode of operation (future use with different generation modes)
	Path            string            `yaml:"Path"`            // Path to fake device folder
	NfdFeatureDir   string            `yaml:"NfdDir"`          // NFD directory
	DevCount        int               `yaml:"DevCount"`        // How many devices to fake
	TilesPerDev     int               `yaml:"TilesPerDev"`     // Per-device tile count
	DevMemSize      int               `yaml:"DevMemSize"`      // Available per-device device-local memory, in bytes
	DevsPerNumaNode int               `yaml:"DevsPerNumaNode"` // How many devices per Numa node
	VfsPerPf        int               `yaml:"VfsPerPf"`        // How many SR-IOV VFs per PF

	// fields for counting what was generated
	files int
	dirs  int
	devs  int
	symls int
}

func addSysfsDriTree(root string, opts *GenOptions, i int) error {
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

	file = filepath.Join(base, "device", "driver")
	if err := os.Symlink(fmt.Sprintf("../../../../bus/pci/drivers/%s", opts.Driver), file); err != nil {
		klog.Fatalf("symlink creation failed '%s': %v",
			file, err)
	}

	opts.symls++

	data = []byte("0x8086")
	file = filepath.Join(base, "device", "vendor")

	if err := os.WriteFile(file, data, fileMode); err != nil {
		return err
	}

	opts.files++

	node := 0
	if opts.DevsPerNumaNode > 0 {
		node = i / opts.DevsPerNumaNode
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

func addSysfsBusTree(root string, opts *GenOptions, i int) error {
	pciName := fmt.Sprintf("0000:00:0%d.0", i)
	base := filepath.Join(root, "bus", "pci", "drivers", opts.Driver, pciName)

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

func addDeviceNodes(base string, opts *GenOptions, i int) error {
	mode := uint32(fileMode | devNullType)
	devid := int(unix.Mkdev(uint32(devNullMajor), uint32(devNullMinor)))

	file := filepath.Join(base, fmt.Sprintf("card%d", cardBase+i))
	if err := unix.Mknod(file, mode, devid); err != nil {
		klog.Fatalf("NULL device (%d:%d) node creation failed for '%s': %v",
			devNullMajor, devNullMinor, file, err)
	}

	opts.devs++

	file = filepath.Join(base, fmt.Sprintf("renderD%d", renderBase+i))
	if err := unix.Mknod(file, mode, devid); err != nil {
		klog.Fatalf("NULL device (%d:%d) node creation failed for '%s': %v",
			devNullMajor, devNullMinor, file, err)
	}

	opts.devs++

	return nil
}

func addDeviceSymlinks(base string, opts *GenOptions, i int) error {
	target := filepath.Join(base, fmt.Sprintf("by-path/pci-0000:%02d:02.0-card", i))
	if err := os.Symlink(fmt.Sprintf("../card%d", cardBase+i), target); err != nil {
		klog.Fatalf("symlink creation failed '%s': %v",
			target, err)
	}

	opts.symls++

	target = filepath.Join(base, fmt.Sprintf("by-path/pci-0000:%02d:02.0-render", i))
	if err := os.Symlink(fmt.Sprintf("../renderD%d", renderBase+i), target); err != nil {
		klog.Fatalf("symlink creation failed '%s': %v",
			target, err)
	}

	opts.symls++

	return nil
}

func addDevfsDriTree(root string, opts *GenOptions, i int) error {
	base := filepath.Join(root, "dri")
	if err := os.MkdirAll(base, dirMode); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(root, "dri/by-path"), dirMode); err != nil {
		return err
	}

	opts.dirs++

	if err := addDeviceNodes(base, opts, i); err != nil {
		return err
	}

	return addDeviceSymlinks(base, opts, i)
}

func addDebugfsDriTree(root string, opts *GenOptions, i int) error {
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
		klog.Fatalf("ReadDir() failed on fake %s path '%s': %v", name, path, err)
	}

	if len(entries) == 0 {
		return
	}

	// This should not be too tight as then node could got blocked just putting entries in to folder
	if name == "sysfs" && len(entries) > 5 {
		klog.Fatalf("too many entries in '%s' - real sysfs?", path)
	}

	// This should not be too tight as then node could got blocked just putting entries in to folder
	if name == "devfs" && (len(entries) > 5) {
		klog.Fatalf("too many entries in '%s', or '%s' != 'dri' - real devfs?", path, entries[0].Name())
	}

	klog.V(1).Infof("Removing already existing fake %s path '%s'", name, path)

	if err = os.RemoveAll(path); err != nil {
		klog.Fatalf("Removing existing %s in '%s' failed: %v", name, path, err)
	}
}

func GenerateDriFiles(opts GenOptions) {
	if opts.Info != "" {
		klog.V(1).Infof("Config: '%s'", opts.Info)
	}

	sysfsPath := opts.Path + sysfsPath
	devfsPath := opts.Path + devfsPath

	removeExistingDir(devfsPath, "devfs")
	removeExistingDir(sysfsPath, "sysfs")
	klog.Infof("Generating fake DRI device(s) sysfs, debugfs and devfs content under '%s' & '%s'",
		sysfsPath, devfsPath)

	opts.dirs, opts.files, opts.devs, opts.symls = 0, 0, 0, 0
	for i := 0; i < opts.DevCount; i++ {
		if err := addSysfsBusTree(sysfsPath, &opts, i); err != nil {
			klog.Fatalf("Dev-%d sysfs bus tree generation failed: %v", i, err)
		}

		if err := addSysfsDriTree(sysfsPath, &opts, i); err != nil {
			klog.Fatalf("Dev-%d sysfs dri tree generation failed: %v", i, err)
		}

		if err := addDevfsDriTree(devfsPath, &opts, i); err != nil {
			klog.Fatalf("Dev-%d devfs tree generation failed: %v", i, err)
		}

		if err := addDebugfsDriTree(sysfsPath, &opts, i); err != nil {
			klog.Fatalf("Dev-%d debugfs tree generation failed: %v", i, err)
		}
	}

	klog.V(1).Infof("Done, created %d dirs, %d devices, %d files and %d symlinks.", opts.dirs, opts.devs, opts.files, opts.symls)

	makeXelinkSideCar(opts)
}

func makeXelinkSideCar(opts GenOptions) {
	topology := opts.Capabilities["connection-topology"]
	gpus := opts.DevCount
	tiles := opts.TilesPerDev
	connections := opts.Capabilities["connections"]

	if topology == fullyConnected {
		saveSideCarFile(opts, buildConnectionList(gpus, tiles))
	} else if connections != "" {
		saveSideCarFile(opts, connections)
	} else {
		return
	}

	klog.V(1).Infof("XELINK: generated xelink sidecar label file, using (GPUs: %d, Tiles: %d, Topology: %s)", gpus, tiles, topology)
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

// saveSideCarFile saves the sidecar labels to a file.
func saveSideCarFile(opts GenOptions, connections string) {
	// Check if the directory exists; if not, use the current directory.
	if opts.NfdFeatureDir == "" || !isDirExists(opts.NfdFeatureDir) {
		klog.Warningf("XELINK: Directory '%s' does not exist. Using current directory.", opts.NfdFeatureDir)
		opts.NfdFeatureDir = "." // Use current directory
	}

	// Kubernetes directory for storing the sidecar labels
	xfile := filepath.Join(opts.NfdFeatureDir, "xpum-sidecar-labels.txt")

	// Create the file
	f, err := os.Create(xfile)
	if err != nil {
		klog.Warningf("XELINK: creation of the xelink sidecar label file '%s' failed: %v", xfile, err)
		return
	}
	defer f.Close()

	klog.V(1).Infof("XELINK: created the xelink sidecar label file '%s'", xfile)

	// Write the main part of the connections string to the file
	line := fmt.Sprintf("xpumanager.intel.com/xe-links=%s", connections[:min(len(connections), maxK8sLabelSize)])
	klog.V(1).Info(line)

	if _, err := f.WriteString(line + "\n"); err != nil {
		klog.Fatalf("XELINK: failed to write to the file '%s': %v", xfile, err)
	}

	// Additional lines for strings longer than maxK8sLabelSize
	index := 2

	for i := maxK8sLabelSize; i < len(connections); i += (maxK8sLabelSize - 1) {
		line := fmt.Sprintf("xpumanager.intel.com/xe-links%d=Z%s", index, connections[i:min(len(connections), i+maxK8sLabelSize-1)])
		klog.V(1).Info(line)

		if _, err := f.WriteString(line + "\n"); err != nil {
			klog.Fatalf("XELINK: failed to write to the file '%s': %v", xfile, err)
		}

		index++
	}

	klog.Infof("XELINK: successfully wrote all labels to '%s'", xfile)
}

// isDirExists checks if a directory exists at the given path.
func isDirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func verifyOptions(opts GenOptions) GenOptions {
	if opts.DevCount < 1 || opts.DevCount > maxDevs {
		klog.Fatalf("Invalid device count: 1 <= %d <= %d", opts.DevCount, maxDevs)
	}

	if opts.VfsPerPf > 0 {
		if opts.TilesPerDev > 0 || opts.DevsPerNumaNode > 0 {
			klog.Fatalf("SR-IOV VFs (%d) with device tiles (%d) or Numa nodes (%d) is unsupported for faking",
				opts.VfsPerPf, opts.TilesPerDev, opts.DevsPerNumaNode)
		}

		if opts.DevCount%(opts.VfsPerPf+1) != 0 {
			klog.Fatalf("%d devices cannot be evenly split to between set of 1 SR-IOV PF + %d VFs",
				opts.DevCount, opts.VfsPerPf)
		}
	}

	if opts.DevsPerNumaNode > opts.DevCount {
		klog.Fatalf("DevsPerNumaNode (%d) > DevCount (%d)", opts.DevsPerNumaNode, opts.DevCount)
	}

	if opts.DevMemSize%mib != 0 {
		klog.Fatalf("Invalid memory size (%f mib), not even mib", float64(opts.DevMemSize)/mib)
	}

	return opts
}

func GetOptionsByJSON(data string) GenOptions {
	if data == "" {
		klog.Fatalf("No fake device spec provided")
	}

	klog.V(1).Infof("Using fake device JSON spec: %v\n", data)

	var opts GenOptions
	if err := json.Unmarshal([]byte(data), &opts); err != nil {
		klog.Fatalf("Unmarshaling JSON spec '%s' failed: %v", data, err)
	}

	return verifyOptions(opts)
}

func GetOptionsByYAML(data string) GenOptions {
	if data == "" {
		klog.Fatalf("No fake device spec provided")
	}

	klog.V(1).Infof("Using fake device YAML spec: %v\n", data)

	var opts GenOptions
	if err := yaml.Unmarshal([]byte(data), &opts); err != nil {
		klog.Fatalf("Unmarshaling YAML spec '%s' failed: %v", data, err)
	}

	return verifyOptions(opts)
}
