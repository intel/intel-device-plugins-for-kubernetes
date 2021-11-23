// Copyright 2020-2021 Intel Corporation. All Rights Reserved.
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
	"bufio"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/pluginutils"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

const (
	labelNamespace     = "gpu.intel.com/"
	gpuListLabelName   = "cards"
	millicoreLabelName = "millicores"
	millicoresPerGPU   = 1000
	memoryOverrideEnv  = "GPU_MEMORY_OVERRIDE"
	memoryReservedEnv  = "GPU_MEMORY_RESERVED"
	gpuDeviceRE        = `^card[0-9]+$`
	controlDeviceRE    = `^controlD[0-9]+$`
	vendorString       = "0x8086"
)

type labelMap map[string]string

type labeler struct {
	gpuDeviceReg     *regexp.Regexp
	controlDeviceReg *regexp.Regexp
	labels           labelMap

	sysfsDRMDir   string
	debugfsDRIDir string
}

func newLabeler(sysfsDRMDir, debugfsDRIDir string) *labeler {
	return &labeler{
		sysfsDRMDir:      sysfsDRMDir,
		debugfsDRIDir:    debugfsDRIDir,
		gpuDeviceReg:     regexp.MustCompile(gpuDeviceRE),
		controlDeviceReg: regexp.MustCompile(controlDeviceRE),
		labels:           labelMap{},
	}
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

func (l *labeler) getMemoryAmount(gpuName string, numTiles uint64) uint64 {
	reserved := getEnvVarNumber(memoryReservedEnv)

	filePath := filepath.Join(l.sysfsDRMDir, gpuName, "lmem_total_bytes")

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

// getTileCount reads the tile count.
func (l *labeler) getTileCount(gpuName string) (numTiles uint64) {
	filePath := filepath.Join(l.sysfsDRMDir, gpuName, "gt/gt*")

	files, _ := filepath.Glob(filePath)

	if len(files) == 0 {
		return 1
	}

	return uint64(len(files))
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

// createCapabilityLabels creates labels from the gpu capability file under debugfs.
func (l *labeler) createCapabilityLabels(cardNum string, numTiles uint64) {
	// try to read the capabilities from the i915_capabilities file
	file, err := os.Open(filepath.Join(l.debugfsDRIDir, cardNum, "i915_capabilities"))
	if err != nil {
		klog.V(3).Infof("Couldn't open file:%s", err.Error()) // debugfs is not stable, there is no need to spam with error level prints
		return
	}
	defer file.Close()

	gen := ""
	media := ""
	graphics := ""
	// define string prefixes to search from the file, and the actions to take in order to create labels from those strings (as funcs)
	searchStringActionMap := map[string]func(string){
		"platform:": func(platformName string) {
			l.labels.addNumericLabel(labelNamespace+"platform_"+platformName+".count", 1)
			l.labels[labelNamespace+"platform_"+platformName+".tiles"] = strconv.FormatInt(int64(numTiles), 10)
			l.labels[labelNamespace+"platform_"+platformName+".present"] = "true"
		},
		// there's also display block version, but that's not relevant
		"media version:": func(version string) {
			l.labels[labelNamespace+"media_version"] = version
			media = version
		},
		"graphics version:": func(version string) {
			l.labels[labelNamespace+"graphics_version"] = version
			graphics = version
		},
		"gen:": func(version string) {
			l.labels[labelNamespace+"platform_gen"] = version
			gen = version
		},
	}

	// Finally, read the file, and try to find the matches. Perform actions and reduce the search map size as we proceed. Return at 0 size.
	scanner := bufio.NewScanner(file)
scanning:
	for scanner.Scan() {
		line := scanner.Text()
		for prefix, action := range searchStringActionMap {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			fields := strings.Split(line, ": ")
			if len(fields) == 2 {
				action(fields[1])
			} else {
				klog.Warningf("invalid '%s' line format: '%s'", file.Name(), line)
			}
			delete(searchStringActionMap, prefix)
			if len(searchStringActionMap) == 0 {
				break scanning
			}
			break
		}
	}
	if gen == "" {
		// TODO: drop gen label before engine types
		// start to have diverging major gen values
		if graphics != "" {
			gen = graphics
		} else if media != "" {
			gen = media
		}
		if gen != "" {
			// truncate to major value
			gen = strings.SplitN(gen, ".", 2)[0]
			l.labels[labelNamespace+"platform_gen"] = gen
		}
	} else if media == "" && graphics == "" {
		// 5.14 or older kernels need this
		l.labels[labelNamespace+"media_version"] = gen
		l.labels[labelNamespace+"graphics_version"] = gen
	}
}

// createLabels is the main function of plugin labeler, it creates label-value pairs for the gpus.
func (l *labeler) createLabels() error {
	gpuNameList, err := l.scan()
	if err != nil {
		return err
	}

	for _, gpuName := range gpuNameList {
		gpuNum := ""
		// extract card number as a string. scan() has already checked name syntax
		_, err = fmt.Sscanf(gpuName, "card%s", &gpuNum)
		if err != nil {
			return errors.Wrap(err, "gpu name parsing error")
		}

		// read the tile count
		numTiles := l.getTileCount(gpuName)

		// read memory amount
		memoryAmount := l.getMemoryAmount(gpuName, numTiles)

		// try to add capability labels
		l.createCapabilityLabels(gpuNum, numTiles)

		l.labels.addNumericLabel(labelNamespace+"memory.max", int64(memoryAmount))
	}
	gpuCount := len(gpuNameList)
	if gpuCount > 0 {
		// add gpu list label (example: "card0.card1.card2")
		l.labels[labelNamespace+gpuListLabelName] = strings.Join(gpuNameList, ".")

		// all GPUs get default number of millicores (1000)
		l.labels.addNumericLabel(labelNamespace+millicoreLabelName, int64(millicoresPerGPU*gpuCount))
	}

	return nil
}

func (l *labeler) printLabels() {
	for key, val := range l.labels {
		fmt.Println(key + "=" + val)
	}
}
