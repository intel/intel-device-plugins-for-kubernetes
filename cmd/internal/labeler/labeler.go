// Copyright 2020-2023 Intel Corporation. All Rights Reserved.
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

package labeler

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/levelzeroservice"
	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/pluginutils"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

const (
	labelNamespace      = "gpu.intel.com/"
	gpuListLabelName    = "cards"
	gpuNumListLabelName = "gpu-numbers"
	millicoreLabelName  = "millicores"
	pciGroupLabelName   = "pci-groups"
	tilesLabelName      = "tiles"
	numaMappingName     = "numa-gpu-map"
	millicoresPerGPU    = 1000
	memoryOverrideEnv   = "GPU_MEMORY_OVERRIDE"
	memoryReservedEnv   = "GPU_MEMORY_RESERVED"
	pciGroupingEnv      = "GPU_PCI_GROUPING_LEVEL"
	gpuDeviceRE         = `^card[0-9]+$`
	controlDeviceRE     = `^controlD[0-9]+$`
	vendorString        = "0x8086"
	labelMaxLength      = 63
	labelControlChar    = "Z"
)

type labelMap map[string]string

type labeler struct {
	gpuDeviceReg     *regexp.Regexp
	controlDeviceReg *regexp.Regexp
	labels           labelMap

	levelzero levelzeroservice.LevelzeroService

	sysfsDRMDir   string
	labelsChanged bool
}

func newLabeler(sysfsDRMDir string) *labeler {
	return &labeler{
		sysfsDRMDir:      sysfsDRMDir,
		gpuDeviceReg:     regexp.MustCompile(gpuDeviceRE),
		controlDeviceReg: regexp.MustCompile(controlDeviceRE),
		labels:           labelMap{},
		labelsChanged:    true,
	}
}

// getPCIPathParts returns a subPath from the given full path starting from folder with prefix "pci".
// returns "" in case not enough folders are found after the one starting with "pci".
func getPCIPathParts(numFolders uint64, fullPath string) string {
	parts := strings.Split(fullPath, "/")

	if len(parts) == 1 {
		return ""
	}

	foundPci := false
	subPath := ""
	separator := ""

	for _, part := range parts {
		if !foundPci && strings.HasPrefix(part, "pci") {
			foundPci = true
		}

		if foundPci && numFolders > 0 {
			subPath = subPath + separator + part
			separator = "/"
			numFolders--
		}

		if numFolders == 0 {
			return subPath
		}
	}

	return ""
}

func (l *labeler) scan() ([]string, error) {
	files, err := os.ReadDir(l.sysfsDRMDir)
	gpuNameList := []string{}

	if err != nil {
		return gpuNameList, errors.Wrap(err, "Can't read sysfs folder")
	}

	for _, f := range files {
		if !l.gpuDeviceReg.MatchString(f.Name()) {
			klog.V(4).Info("Not compatible device", f.Name())
			continue
		}

		dat, err := os.ReadFile(path.Join(l.sysfsDRMDir, f.Name(), "device/vendor"))
		if err != nil {
			klog.Warning("Skipping. Can't read vendor file: ", err)
			continue
		}

		if strings.TrimSpace(string(dat)) != vendorString {
			klog.V(4).Info("Non-Intel GPU", f.Name())
			continue
		}

		if pluginutils.IsSriovPFwithVFs(path.Join(l.sysfsDRMDir, f.Name())) {
			klog.V(4).Infof("Skipping PF with VF")
			continue
		}

		_, err = os.ReadDir(path.Join(l.sysfsDRMDir, f.Name(), "device/drm"))
		if err != nil {
			return gpuNameList, errors.Wrap(err, "Can't read device folder")
		}

		gpuNameList = append(gpuNameList, f.Name())
	}

	return gpuNameList, nil
}

func getEnvVarNumber(envVarName string) uint64 {
	envValue := os.Getenv(envVarName)
	if envValue != "" {
		val, err := strconv.ParseUint(envValue, 10, 64)
		if err == nil {
			return val
		}
	}

	return 0
}

func fallback() uint64 {
	return getEnvVarNumber(memoryOverrideEnv)
}

func legacyFallback(sysfsDrmDir, gpuName string, numTiles uint64) uint64 {
	reserved := getEnvVarNumber(memoryReservedEnv)

	filePath := filepath.Join(sysfsDrmDir, gpuName, "lmem_total_bytes")

	dat, err := os.ReadFile(filePath)
	if err != nil {
		klog.Warning("Can't read file: ", err)
		return fallback()
	}

	totalPerTile, err := strconv.ParseUint(strings.TrimSpace(string(dat)), 0, 64)
	if err != nil {
		klog.Warning("Can't convert lmem_total_bytes: ", err)
		return fallback()
	}

	return totalPerTile*numTiles - reserved
}

func (l *labeler) GetMemoryAmount(sysfsDrmDir, gpuName string, numTiles uint64) uint64 {
	link, err := os.Readlink(filepath.Join(sysfsDrmDir, gpuName, "device"))
	if err != nil {
		return legacyFallback(sysfsDrmDir, gpuName, numTiles)
	}

	amount := uint64(0)

	if l.levelzero != nil {
		amount, err = l.levelzero.GetDeviceMemoryAmount(filepath.Base(link))
		if amount == 0 || err != nil {
			return legacyFallback(sysfsDrmDir, gpuName, numTiles)
		}
	} else {
		return legacyFallback(sysfsDrmDir, gpuName, numTiles)
	}

	return amount
}

// GetTileCount reads the tile count.
func GetTileCount(cardPath string) (numTiles uint64) {
	files := []string{}

	paths, _ := filepath.Glob(filepath.Join(cardPath, "gt/gt*")) // i915 driver
	files = append(files, paths...)

	paths, _ = filepath.Glob(filepath.Join(cardPath, "device/tile?")) // Xe driver
	files = append(files, paths...)

	klog.V(4).Info("tile files found:", files)

	if len(files) == 0 {
		return 1
	}

	return uint64(len(files))
}

// GetNumaNode reads the cards numa node.
func GetNumaNode(sysfsDrmDir, gpuName string) int {
	filePath := filepath.Join(sysfsDrmDir, gpuName, "device/numa_node")

	data, err := os.ReadFile(filePath)
	if err != nil {
		klog.Warning("Can't read file: ", err)
		return -1
	}

	numa, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		klog.Warning("Can't convert numa_node: ", err)
		return -1
	}

	if numa > math.MaxInt16 {
		klog.Warning("Too large numa: ", numa)

		return -1
	}

	return int(numa)
}

// addNumericLabel creates a new label if one doesn't exist. Else the new value is added to the previous value.
func (lm labelMap) addNumericLabel(labelName string, valueToAdd int64) {
	value := int64(0)
	if numstr, ok := lm[labelName]; ok {
		_, _ = fmt.Sscanf(numstr, "%d", &value)
	}

	value += valueToAdd
	lm[labelName] = strconv.FormatInt(value, 10)
}

// Stores a long string to labels so that it's possibly split into multiple
// keys: foobar="<something very long>", foobar2="<equally long>", foobar3="The end."
func (lm labelMap) addSplittableString(labelBase, fullValue string) {
	splitList := pluginutils.SplitAtLastAlphaNum(fullValue, labelMaxLength, labelControlChar)

	lm[labelBase] = splitList[0]

	for i := 1; i < len(splitList); i++ {
		nextLabel := labelBase + strconv.FormatInt(int64(i+1), 10)
		lm[nextLabel] = splitList[i]
	}
}

// this returns PCI groups label value, groups separated by "_", gpus separated by ".".
// Example for two groups with 4 gpus: "0.1.2.3_4.5.6.7".
func (l *labeler) createPCIGroupLabel(gpuNumList []string) string {
	pciGroups := map[string][]string{}

	pciGroupLevel := getEnvVarNumber(pciGroupingEnv)
	if pciGroupLevel == 0 {
		return ""
	}

	for _, gpuNum := range gpuNumList {
		symLinkTarget, err := filepath.EvalSymlinks(path.Join(l.sysfsDRMDir, "card"+gpuNum))

		if err == nil {
			if pathPart := getPCIPathParts(pciGroupLevel, symLinkTarget); pathPart != "" {
				pciGroups[pathPart] = append(pciGroups[pathPart], gpuNum)
			}
		}
	}

	labelValue := ""
	separator := ""

	// process in stable order by sorting
	keys := []string{}
	for key := range pciGroups {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		labelValue = labelValue + separator + strings.Join(pciGroups[key], ".")
		separator = "_"
	}

	return labelValue
}

// createLabels is the main function of plugin labeler, it creates label-value pairs for the gpus.
func (l *labeler) createLabels() error {
	prevLabels := l.labels

	l.labels = labelMap{}

	gpuNameList, err := l.scan()
	if err != nil {
		return err
	}

	gpuNumList := []string{}
	tileCount := 0

	numaMapping := make(map[int][]string)

	for _, gpuName := range gpuNameList {
		gpuNum := ""
		// extract gpu number as a string. scan() has already checked name syntax
		_, err = fmt.Sscanf(gpuName, "card%s", &gpuNum)
		if err != nil {
			return errors.Wrap(err, "gpu name parsing error")
		}

		numTiles := GetTileCount(filepath.Join(l.sysfsDRMDir, gpuName))
		tileCount += int(numTiles)

		memoryAmount := l.GetMemoryAmount(l.sysfsDRMDir, gpuName, numTiles)
		gpuNumList = append(gpuNumList, gpuName[4:])

		// get numa node of the GPU
		numaNode := GetNumaNode(l.sysfsDRMDir, gpuName)

		if numaNode >= 0 {
			// and store the gpu under that node id
			numaList := numaMapping[numaNode]
			numaList = append(numaList, gpuNum)

			numaMapping[numaNode] = numaList
		}

		if memoryAmount < math.MaxInt64 {
			l.labels.addNumericLabel(labelNamespace+"memory.max", int64(memoryAmount))
		}
	}

	gpuCount := len(gpuNumList)

	l.labels.addNumericLabel(labelNamespace+tilesLabelName, int64(tileCount))

	if gpuCount > 0 {
		// add gpu list label (example: "card0.card1.card2") - deprecated
		l.labels[labelNamespace+gpuListLabelName] = pluginutils.SplitAtLastAlphaNum(
			strings.Join(gpuNameList, "."), labelMaxLength, labelControlChar)[0]

		// add gpu num list label(s) (example: "0.1.2", which is short form of "card0.card1.card2")
		l.labels.addSplittableString(labelNamespace+gpuNumListLabelName, strings.Join(gpuNumList, "."))

		if len(numaMapping) > 0 {
			// add numa node mapping to labels: gpu.intel.com/numa-gpu-map="0-0.1.2.3_1-4.5.6.7"
			numaMappingLabel := createNumaNodeMappingLabel(numaMapping)

			l.labels.addSplittableString(labelNamespace+numaMappingName, numaMappingLabel)
		}

		// all GPUs get default number of millicores (1000)
		l.labels.addNumericLabel(labelNamespace+millicoreLabelName, int64(millicoresPerGPU*gpuCount))

		// aa pci-group label(s), (two group example: "1.2.3.4_5.6.7.8")
		allPCIGroups := l.createPCIGroupLabel(gpuNumList)
		if allPCIGroups != "" {
			l.labels.addSplittableString(labelNamespace+pciGroupLabelName, allPCIGroups)
		}
	}

	l.labelsChanged = !reflect.DeepEqual(prevLabels, l.labels)

	return nil
}

func createNumaNodeMappingLabel(mapping map[int][]string) string {
	parts := []string{}

	numas := []int{}
	for numaNode := range mapping {
		numas = append(numas, numaNode)
	}

	sort.Ints(numas)

	for _, numaNode := range numas {
		gpus := mapping[numaNode]
		numaString := strconv.FormatInt(int64(numaNode), 10)
		gpusString := strings.Join(gpus, ".")

		parts = append(parts, numaString+"-"+gpusString)
	}

	return strings.Join(parts, "_")
}

func (l *labeler) atomicPrintLabelsToFile(labelFile string) error {
	baseDir := filepath.Dir(labelFile)

	// TODO: Use NFD's "hidden file" feature when it becomes available.
	d, err := os.MkdirTemp(baseDir, "labels")
	if err != nil {
		klog.Warning("could not create temporary directory, writing directly to destination")

		return l.printLabelsToFile(labelFile)
	}

	defer os.RemoveAll(d)

	tmpFile := filepath.Join(d, "labels.txt")

	if err := l.printLabelsToFile(tmpFile); err != nil {
		return err
	}

	return os.Rename(tmpFile, labelFile)
}

func (l *labeler) printLabelsToFile(labelFile string) error {
	f, err := os.OpenFile(labelFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file (%s): %w", labelFile, err)
	}

	defer f.Close()

	for key, val := range l.labels {
		if _, err := f.WriteString(key + "=" + val + "\n"); err != nil {
			return fmt.Errorf("failed to write label (%s=%s) to file: %w", key, val, err)
		}
	}

	return nil
}

func CreateAndPrintLabels(sysfsDRMDir string) {
	l := newLabeler(sysfsDRMDir)

	if err := l.createLabels(); err != nil {
		klog.Warningf("failed to create labels: %+v", err)

		return
	}

	for key, val := range l.labels {
		fmt.Println(key + "=" + val)
	}
}

// Gathers node's GPU labels on channel trigger or timeout, and write them to a file.
// The created label file is deleted on exit (process dying).
func Run(sysfsDrmDir, nfdFeatureFile string, updateInterval time.Duration, scanResources chan bool, levelzero levelzeroservice.LevelzeroService, exitFunc func()) {
	l := newLabeler(sysfsDrmDir)

	l.levelzero = levelzero

	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)

	klog.V(1).Info("Starting GPU labeler")

Loop:
	for {
		timeout := time.After(updateInterval)

		select {
		case <-timeout:
		case <-scanResources:
		case interrupt := <-interruptChan:
			klog.V(2).Infof("Interrupt %d received", interrupt)

			break Loop
		}

		klog.V(1).Info("Ext resources scanning")

		err := l.createLabels()
		if err != nil {
			klog.Warningf("label creation failed: %+v", err)

			continue
		}

		if l.labelsChanged {
			klog.V(1).Info("Writing labels")

			if err := l.atomicPrintLabelsToFile(nfdFeatureFile); err != nil {
				klog.Warningf("failed to write labels to file: %+v", err)

				// Reset labels so that next time the labeler runs the writing is retried.
				l.labels = labelMap{}
			}
		}
	}

	signal.Stop(interruptChan)

	klog.V(2).Info("Removing label file")

	err := os.Remove(nfdFeatureFile)
	if err != nil {
		klog.Errorf("Failed to cleanup label file: %+v", err)
	}

	klog.V(1).Info("Stopping GPU labeler")

	// Call exitFunc that might exit the app
	exitFunc()
}
