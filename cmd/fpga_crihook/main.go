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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/bitstream"
	fpga "github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/linux"
	"github.com/pkg/errors"
	utilsexec "k8s.io/utils/exec"
)

const (
	fpgaBitStreamDirectory = "/srv/intel.com/fpga"
	configJSON             = "config.json"
	fpgaRegionEnvPrefix    = "FPGA_REGION_"
	fpgaAfuEnvPrefix       = "FPGA_AFU_"

	annotationName  = "com.intel.fpga.mode"
	annotationValue = "fpga.intel.com/region"
)

// Stdin defines structure for standard JSONed input of the OCI platform hook
type Stdin struct {
	Annotations struct {
		ComIntelFpgaMode string `json:"com.intel.fpga.mode"`
	} `json:"annotations"`
	Bundle string `json:"bundle"`
}

// Config defines structure of OCI hook configuration
type Config struct {
	Process struct {
		Env []string `json:"env"`
	} `json:"process"`
	Linux struct {
		Resources struct {
			Devices []struct {
				Major int `json:"major,omitempty"`
				Minor int `json:"minor,omitempty"`
			} `json:"devices"`
		} `json:"resources"`
		Devices []Device `json:"devices"`
	} `json:"linux"`
}

// Device defines structure for Config.Linux.Devices entries
type Device struct {
	// these fields are set by the hook
	name      string
	processed bool

	// these are set by JSON decoder
	Path  string `json:"path"`
	Type  string `json:"type"`
	Major int    `json:"major"`
	Minor int    `json:"minor"`
	UID   int    `json:"uid"`
	Gid   int    `json:"gid"`
}

func (dev *Device) getName() string {
	if len(dev.name) == 0 {
		dev.name = filepath.Base(dev.Path)
	}
	return dev.name
}

func decodeJSONStream(reader io.Reader, dest interface{}) error {
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&dest)
	return errors.WithStack(err)
}

type hookEnv struct {
	sysFsPrefix  string
	bitstreamDir string
	config       string
	execer       utilsexec.Interface
}

type fpgaParams struct {
	region     string
	afu        string
	portDevice string
}

func newHookEnv(sysFsPrefix, bitstreamDir string, config string, execer utilsexec.Interface) (*hookEnv, error) {
	return &hookEnv{
		sysFsPrefix:  sysFsPrefix,
		bitstreamDir: bitstreamDir,
		config:       config,
		execer:       execer,
	}, nil
}

func (he *hookEnv) getConfig(stdinJ *Stdin) (*Config, error) {
	configPath := filepath.Join(stdinJ.Bundle, he.config)
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer configFile.Close()

	var config Config
	err = decodeJSONStream(configFile, &config)
	if err != nil {
		return nil, errors.WithMessage(err, "can't decode "+configPath)
	}

	if len(config.Process.Env) == 0 {
		return nil, errors.Errorf("%s: process.env is empty", configPath)
	}

	if len(config.Linux.Devices) == 0 {
		return nil, errors.Errorf("%s: linux.devices is empty", configPath)
	}
	return &config, nil
}

func (he *hookEnv) getFPGAParams(config *Config) ([]fpgaParams, error) {
	// parse FPGA_REGION_N and FPGA_AFU_N environment variables
	regionEnv := make(map[string]string)
	afuEnv := make(map[string]string)
	for _, env := range config.Process.Env {
		splitted := strings.SplitN(env, "=", 2)
		if strings.HasPrefix(splitted[0], fpgaRegionEnvPrefix) {
			num := strings.Split(splitted[0], fpgaRegionEnvPrefix)[1]
			regionEnv[num] = fpga.CanonizeID(splitted[1])
		} else if strings.HasPrefix(splitted[0], fpgaAfuEnvPrefix) {
			num := strings.Split(splitted[0], fpgaAfuEnvPrefix)[1]
			afuEnv[num] = fpga.CanonizeID(splitted[1])
		}
	}

	if len(regionEnv) == 0 {
		return nil, errors.Errorf("No %s* environment variables are set", fpgaRegionEnvPrefix)
	}

	if len(afuEnv) == 0 {
		return nil, errors.Errorf("No %s* environment variables are set", fpgaAfuEnvPrefix)
	}

	if len(afuEnv) != len(regionEnv) {
		return nil, errors.Errorf("Environment variables %s* and %s* don't match", fpgaRegionEnvPrefix, fpgaAfuEnvPrefix)
	}

	params := []fpgaParams{}
	for num, region := range regionEnv {
		afu, ok := afuEnv[num]
		if !ok {
			return nil, errors.Errorf("Environment variable %s%s is not set", fpgaAfuEnvPrefix, num)
		}

		// Find a device suitable for the region/interface id
		found := false
		for _, dev := range config.Linux.Devices {
			deviceName := dev.getName()
			// skip non-FPGA devices
			if !fpga.IsFpgaPort(deviceName) {
				continue
			}

			// skip already processed devices
			if dev.processed {
				continue
			}
			port, err := fpga.NewFpgaPort(deviceName)
			if err != nil {
				return nil, err
			}
			if interfaceUUID := port.GetInterfaceUUID(); interfaceUUID == region {
				params = append(params,
					fpgaParams{
						afu:        afu,
						region:     interfaceUUID,
						portDevice: deviceName,
					},
				)
				dev.processed = true
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("can't find appropriate device for region %s", region)
		}
	}
	return params, nil
}

func getStdin(reader io.Reader) (*Stdin, error) {
	var stdinJ Stdin
	err := decodeJSONStream(reader, &stdinJ)
	if err != nil {
		return nil, err
	}

	// Check if device plugin annotation is set
	if stdinJ.Annotations.ComIntelFpgaMode == "" {
		return nil, fmt.Errorf("annotation %s is not set", annotationName)
	}

	// Check if device plugin annotation is set
	if stdinJ.Annotations.ComIntelFpgaMode != annotationValue {
		return nil, fmt.Errorf("annotation %s has incorrect value '%s'", annotationName, stdinJ.Annotations.ComIntelFpgaMode)
	}

	if stdinJ.Bundle == "" {
		return nil, errors.New("'bundle' field is not set in the stdin JSON")
	}

	if _, err := os.Stat(stdinJ.Bundle); err != nil {
		return nil, fmt.Errorf("bundle directory %s: stat error: %+v", stdinJ.Bundle, err)
	}

	return &stdinJ, nil
}

func (he *hookEnv) process(reader io.Reader) error {
	stdin, err := getStdin(reader)
	if err != nil {
		return err
	}

	config, err := he.getConfig(stdin)
	if err != nil {
		return err
	}

	paramslist, err := he.getFPGAParams(config)
	if err != nil {
		return errors.WithMessage(err, "couldn't get FPGA region, AFU and device node")
	}

	for _, params := range paramslist {
		port, err := fpga.NewFpgaPort(params.portDevice)
		if err != nil {
			return err
		}

		programmedAfu := port.GetAcceleratorTypeUUID()
		if programmedAfu == params.afu {
			// Afu is already programmed
			return nil
		}

		bitstream, err := bitstream.GetFPGABitstream(he.bitstreamDir, params.region, params.afu)
		if err != nil {
			return err
		}
		defer bitstream.Close()

		err = port.PR(bitstream, false)

		programmedAfu = port.GetAcceleratorTypeUUID()

		if programmedAfu != bitstream.AcceleratorTypeUUID() {
			return errors.Errorf("programmed function %s instead of %s", programmedAfu, bitstream.AcceleratorTypeUUID())
		}
	}

	return nil
}

func main() {
	if os.Getenv("PATH") == "" { // runc doesn't set PATH when runs hooks
		os.Setenv("PATH", "/sbin:/usr/sbin:/usr/local/sbin:/usr/local/bin:/usr/bin:/bin")
	}

	he, err := newHookEnv("", fpgaBitStreamDirectory, configJSON, utilsexec.New())
	if err == nil {
		err = he.process(os.Stdin)
	}

	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}
