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
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/golang/glog"
	utilsexec "k8s.io/utils/exec"
)

const (
	fpgaBitStreamDirectory = "/srv/intel.com/fpga"
	configJSON             = "config.json"
	fpgaRegionEnv          = "FPGA_REGION"
	fpgaAfuEnv             = "FPGA_AFU"
	fpgaBitStreamExt       = ".gbs"
)

func decodeJSONStream(reader io.Reader) (map[string]interface{}, error) {
	decoder := json.NewDecoder(reader)
	content := make(map[string]interface{})
	err := decoder.Decode(&content)
	return content, err
}

type hookEnv struct {
	bitStreamDir string
	config       string
	execer       utilsexec.Interface
}

func newHookEnv(bitStreamDir string, config string, execer utilsexec.Interface) *hookEnv {
	return &hookEnv{
		bitStreamDir,
		config,
		execer,
	}
}

func canonize(uuid string) string {
	return strings.ToLower(strings.Replace(uuid, "-", "", -1))
}

func (he *hookEnv) getFPGAParams(reader io.Reader) (string, string, error) {
	content, err := decodeJSONStream(reader)
	if err != nil {
		return "", "", err
	}

	bundle, ok := content["bundle"]
	if !ok {
		return "", "", fmt.Errorf("no 'bundle' field in the configuration")
	}

	configPath := path.Join(fmt.Sprint(bundle), he.config)
	configFile, err := os.Open(configPath)
	if err != nil {
		return "", "", err
	}
	defer configFile.Close()

	content, err = decodeJSONStream(configFile)
	if err != nil {
		return "", "", fmt.Errorf("can't decode %s", configPath)
	}

	process, ok := content["process"]
	if !ok {
		return "", "", fmt.Errorf("no 'process' field found in %s", configPath)
	}

	rawEnv, ok := process.(map[string]interface{})["env"]
	if !ok {
		return "", "", fmt.Errorf("no 'env' field found in the 'process' struct in %s", configPath)
	}

	dEnv := make(map[string]string)
	for _, env := range rawEnv.([]interface{}) {
		splitted := strings.SplitN(env.(string), "=", 2)
		dEnv[splitted[0]] = splitted[1]
	}

	fpgaRegion, ok := dEnv[fpgaRegionEnv]
	if !ok {
		return "", "", fmt.Errorf("%s environment is not set in the 'process/env' list in %s", fpgaRegionEnv, configPath)
	}

	fpgaAfu, ok := dEnv[fpgaAfuEnv]
	if !ok {
		return fpgaRegion, "", fmt.Errorf("%s environment is not set in the 'process/env' list in %s", fpgaAfuEnv, configPath)
	}

	return canonize(fpgaRegion), canonize(fpgaAfu), nil
}

func (he *hookEnv) validateBitStream(fpgaRegion string, fpgaAfu string, fpgaBitStreamPath string) error {
	output, err := he.execer.Command("packager", "gbs-info", "--gbs", fpgaBitStreamPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s/%s: can't get bitstream info: %v", fpgaRegion, fpgaAfu, err)
	}

	reader := bytes.NewBuffer(output)
	content, err := decodeJSONStream(reader)
	if err != nil {
		return fmt.Errorf("%s/%s: can't decode 'packager gbs-info' output: %v", fpgaRegion, fpgaAfu, err)
	}

	afuImage, ok := content["afu-image"]
	if !ok {
		return fmt.Errorf("%s/%s: 'afu-image' field not found in the 'packager gbs-info' output", fpgaRegion, fpgaAfu)
	}

	interfaceUUID, ok := afuImage.(map[string]interface{})["interface-uuid"]
	if !ok {
		return fmt.Errorf("%s/%s: 'interface-uuid' field not found in the 'packager gbs-info' output", fpgaRegion, fpgaAfu)
	}

	acceleratorClusters, ok := afuImage.(map[string]interface{})["accelerator-clusters"]
	if !ok {
		return fmt.Errorf("%s/%s: 'accelerator-clusters' field not found in the 'packager gbs-info' output", fpgaRegion, fpgaAfu)
	}

	if canonize(interfaceUUID.(string)) != canonize(fpgaRegion) {
		return fmt.Errorf("bitstream is not for this device: region(%s) and interface-uuid(%s) don't match", fpgaRegion, interfaceUUID)
	}

	acceleratorTypeUUID, ok := acceleratorClusters.([]interface{})[0].(map[string]interface{})["accelerator-type-uuid"]
	if !ok {
		return fmt.Errorf("%s/%s: 'accelerator-type-uuid' field not found in the 'packager gbs-info' output", fpgaRegion, fpgaAfu)
	}

	if canonize(acceleratorTypeUUID.(string)) != canonize(fpgaAfu) {
		return fmt.Errorf("incorrect bitstream: AFU(%s) and accelerator-type-uuid(%s) don't match", fpgaAfu, acceleratorTypeUUID)
	}

	return nil
}

func (he *hookEnv) programBitStream(fpgaRegion string, fpgaAfu string, fpgaBitStreamPath string) error {
	output, err := he.execer.Command("fpgaconf", fpgaBitStreamPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to program AFU %s to region %s: error: %v, output: %s", fpgaAfu, fpgaRegion, err, string(output))
	}
	return nil
}

func (he *hookEnv) process(reader io.Reader) error {
	fpgaRegion, fpgaAfu, err := he.getFPGAParams(reader)
	if err != nil {
		return fmt.Errorf("couldn't get FPGA region and AFU: %v, skipping", err)
	}

	fpgaBitStreamPath := path.Join(he.bitStreamDir, fpgaRegion, fpgaAfu+fpgaBitStreamExt)
	if _, err = os.Stat(fpgaBitStreamPath); os.IsNotExist(err) {
		return fmt.Errorf("%s/%s: bitstream is not found", fpgaRegion, fpgaAfu)
	}

	err = he.validateBitStream(fpgaRegion, fpgaAfu, fpgaBitStreamPath)
	if err != nil {
		return err
	}

	err = he.programBitStream(fpgaRegion, fpgaAfu, fpgaBitStreamPath)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	//work around glog ERROR: logging before flag.Parse: I0618
	flag.Parse()

	if os.Getenv("PATH") == "" { // runc doesn't set PATH when runs hooks
		os.Setenv("PATH", "/sbin:/usr/sbin:/usr/local/sbin:/usr/local/bin:/usr/bin:/bin")
	}

	he := newHookEnv(fpgaBitStreamDirectory, configJSON, utilsexec.New())

	err := he.process(os.Stdin)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}
}
