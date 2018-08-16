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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	utilsexec "k8s.io/utils/exec"
)

const (
	fpgaBitStreamDirectory = "/srv/intel.com/fpga"
	configJSON             = "config.json"
	fpgaRegionEnv          = "FPGA_REGION"
	fpgaAfuEnv             = "FPGA_AFU"
	fpgaBitStreamExt       = ".gbs"
	fpgaDevRegexp          = `\/dev\/intel-fpga-port.(\d)$`
	afuIDTemplate          = "/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id"
	annotationName         = "com.intel.fpga.mode"
	annotationValue        = "fpga.intel.com/region"
)

func decodeJSONStream(reader io.Reader) (map[string]interface{}, error) {
	decoder := json.NewDecoder(reader)
	content := make(map[string]interface{})
	err := decoder.Decode(&content)
	return content, errors.WithStack(err)
}

type hookEnv struct {
	bitStreamDir  string
	config        string
	execer        utilsexec.Interface
	afuIDTemplate string
}

type fpgaParams struct {
	region string
	afu    string
	devNum string
}

func newHookEnv(bitStreamDir string, config string, execer utilsexec.Interface, afuIDTemplate string) *hookEnv {
	return &hookEnv{
		bitStreamDir,
		config,
		execer,
		afuIDTemplate,
	}
}

func canonize(uuid string) string {
	return strings.ToLower(strings.Replace(uuid, "-", "", -1))
}

func (he *hookEnv) getFPGAParams(content map[string]interface{}) (*fpgaParams, error) {
	bundle, ok := content["bundle"]
	if !ok {
		return nil, errors.New("no 'bundle' field in the configuration")
	}

	configPath := path.Join(fmt.Sprint(bundle), he.config)
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer configFile.Close()

	content, err = decodeJSONStream(configFile)
	if err != nil {
		return nil, errors.WithMessage(err, "can't decode "+configPath)
	}

	process, ok := content["process"]
	if !ok {
		return nil, errors.Errorf("no 'process' field found in %s", configPath)
	}

	rawEnv, ok := process.(map[string]interface{})["env"]
	if !ok {
		return nil, errors.Errorf("no 'env' field found in the 'process' struct in %s", configPath)
	}

	dEnv := make(map[string]string)
	for _, env := range rawEnv.([]interface{}) {
		splitted := strings.SplitN(env.(string), "=", 2)
		dEnv[splitted[0]] = splitted[1]
	}

	fpgaRegion, ok := dEnv[fpgaRegionEnv]
	if !ok {
		return nil, errors.Errorf("%s environment is not set in the 'process/env' list in %s", fpgaRegionEnv, configPath)
	}

	fpgaAfu, ok := dEnv[fpgaAfuEnv]
	if !ok {
		return nil, errors.Errorf("%s environment is not set in the 'process/env' list in %s", fpgaAfuEnv, configPath)
	}

	linux, ok := content["linux"]
	if !ok {
		return nil, errors.Errorf("no 'linux' field found in %s", configPath)
	}

	rawDevices, ok := linux.(map[string]interface{})["devices"]
	if !ok {
		return nil, errors.Errorf("no 'devices' field found in the 'linux' struct in %s", configPath)
	}

	pattern := regexp.MustCompile(fpgaDevRegexp)
	for _, device := range rawDevices.([]interface{}) {
		deviceNum := pattern.FindStringSubmatch(device.(map[string]interface{})["path"].(string))
		if deviceNum != nil {
			return &fpgaParams{region: canonize(fpgaRegion), afu: canonize(fpgaAfu), devNum: deviceNum[1]}, nil
		}
	}

	return nil, errors.Errorf("no FPGA devices found in linux/devices list in %s", configPath)

}

func (he *hookEnv) validateBitStream(params *fpgaParams, fpgaBitStreamPath string) error {
	output, err := he.execer.Command("packager", "gbs-info", "--gbs", fpgaBitStreamPath).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "%s/%s: can't get bitstream info", params.region, params.afu)
	}

	reader := bytes.NewBuffer(output)
	content, err := decodeJSONStream(reader)
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("%s/%s: can't decode 'packager gbs-info' output", params.region, params.afu))
	}

	afuImage, ok := content["afu-image"]
	if !ok {
		return errors.Errorf("%s/%s: 'afu-image' field not found in the 'packager gbs-info' output", params.region, params.afu)
	}

	interfaceUUID, ok := afuImage.(map[string]interface{})["interface-uuid"]
	if !ok {
		return errors.Errorf("%s/%s: 'interface-uuid' field not found in the 'packager gbs-info' output", params.region, params.afu)
	}

	acceleratorClusters, ok := afuImage.(map[string]interface{})["accelerator-clusters"]
	if !ok {
		return errors.Errorf("%s/%s: 'accelerator-clusters' field not found in the 'packager gbs-info' output", params.region, params.afu)
	}

	if canonize(interfaceUUID.(string)) != params.region {
		return errors.Errorf("bitstream is not for this device: region(%s) and interface-uuid(%s) don't match", params.region, interfaceUUID)
	}

	acceleratorTypeUUID, ok := acceleratorClusters.([]interface{})[0].(map[string]interface{})["accelerator-type-uuid"]
	if !ok {
		return errors.Errorf("%s/%s: 'accelerator-type-uuid' field not found in the 'packager gbs-info' output", params.region, params.afu)
	}

	if canonize(acceleratorTypeUUID.(string)) != params.afu {
		return errors.Errorf("incorrect bitstream: AFU(%s) and accelerator-type-uuid(%s) don't match", params.afu, acceleratorTypeUUID)
	}

	return nil
}

func (he *hookEnv) programBitStream(params *fpgaParams, fpgaBitStreamPath string) error {
	output, err := he.execer.Command("fpgaconf", "-S", params.devNum, fpgaBitStreamPath).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to program AFU %s to socket %s, region %s: output: %s", params.afu, params.devNum, params.region, string(output))
	}

	programmedAfu, err := he.getProgrammedAfu(params.devNum)
	if err != nil {
		return err
	}

	if programmedAfu != params.afu {
		return errors.Errorf("programmed function %s instead of %s", programmedAfu, params.afu)
	}

	return nil
}

func (he *hookEnv) getProgrammedAfu(deviceNum string) (string, error) {
	// NOTE: only one region per device is supported, hence
	// deviceNum is used twice (device and port numbers are the same)
	afuIDPath := fmt.Sprintf(he.afuIDTemplate, deviceNum, deviceNum)
	data, err := ioutil.ReadFile(afuIDPath)
	if err != nil {
		return "", errors.WithStack(err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (he *hookEnv) process(reader io.Reader) error {
	content, err := decodeJSONStream(reader)
	if err != nil {
		return err
	}

	// Check if device plugin annotation is set
	annotations, ok := content["annotations"]
	if !ok {
		return errors.New("no 'annotations' field in the configuration")
	}

	annotation, ok := annotations.(map[string]interface{})[annotationName]
	if !ok {
		fmt.Printf("annotation %s is not set, skipping\n", annotationName)
		return nil
	}
	if annotation != annotationValue {
		fmt.Printf("annotation %s has incorrect value, skipping\n", annotationName)
		return nil
	}

	params, err := he.getFPGAParams(content)
	if err != nil {
		return errors.WithMessage(err, "couldn't get FPGA region, AFU and device number")
	}

	programmedAfu, err := he.getProgrammedAfu(params.devNum)
	if err != nil {
		return err
	}

	if canonize(programmedAfu) == params.afu {
		// Afu is already programmed
		return nil
	}

	fpgaBitStreamPath := path.Join(he.bitStreamDir, params.region, params.afu+fpgaBitStreamExt)
	if _, err = os.Stat(fpgaBitStreamPath); os.IsNotExist(err) {
		return errors.Errorf("%s/%s: bitstream is not found", params.region, params.afu)
	}

	err = he.validateBitStream(params, fpgaBitStreamPath)
	if err != nil {
		return err
	}

	err = he.programBitStream(params, fpgaBitStreamPath)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	if os.Getenv("PATH") == "" { // runc doesn't set PATH when runs hooks
		os.Setenv("PATH", "/sbin:/usr/sbin:/usr/local/sbin:/usr/local/bin:/usr/bin:/bin")
	}

	he := newHookEnv(fpgaBitStreamDirectory, configJSON, utilsexec.New(), afuIDTemplate)

	err := he.process(os.Stdin)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}
