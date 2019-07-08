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
	// "bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/gbs"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	utilsexec "k8s.io/utils/exec"
	// "github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga/aocx"
)

const (
	fpgaBitStreamDirectory = "/srv/intel.com/fpga"
	packager               = "/opt/intel/fpga-sw/opae/bin/packager"
	fpgaconf               = "/opt/intel/fpga-sw/opae/fpgaconf-wrapper"
	aocl                   = "/opt/intel/fpga-sw/opencl/aocl-wrapper"
	configJSON             = "config.json"
	fpgaRegionEnvPrefix    = "FPGA_REGION_"
	fpgaAfuEnvPrefix       = "FPGA_AFU_"
	fpgaDevRegexp          = `\/dev\/intel-fpga-port.(\d)$`
	afuIDTemplate          = "/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-port.%s/afu_id"
	interfaceIDTemplate    = "/sys/class/fpga/intel-fpga-dev.%s/intel-fpga-fme.%s/pr/interface_id"
	annotationName         = "com.intel.fpga.mode"
	annotationValue        = "fpga.intel.com/region"
	pciAddressRegex        = `^[[:xdigit:]]{4}:([[:xdigit:]]{2}):([[:xdigit:]]{2})\.([[:xdigit:]])$`
)

var (
	fpgaDevRE    = regexp.MustCompile(fpgaDevRegexp)
	pciAddressRE = regexp.MustCompile(pciAddressRegex)
)

func decodeJSONStream(reader io.Reader) (map[string]interface{}, error) {
	decoder := json.NewDecoder(reader)
	content := make(map[string]interface{})
	err := decoder.Decode(&content)
	return content, errors.WithStack(err)
}

type hookEnv struct {
	bitStreamDir        string
	config              string
	execer              utilsexec.Interface
	afuIDTemplate       string
	interfaceIDTemplate string
}

type fpgaParams struct {
	region string
	afu    string
	devNum string
}

type fpgaBitStream interface {
	validate() error
	program() error
}

type opaeBitStream struct {
	path   string
	params fpgaParams
	execer utilsexec.Interface
}

func (bitStream *opaeBitStream) validate() error {
	region, afu := bitStream.params.region, bitStream.params.afu

	gbs, err := gbs.Open(bitStream.path)
	if err != nil {
		return errors.Wrapf(err, "%s/%s: can't get bitstream info", region, afu)
	}

	if canonize(gbs.Metadata.AfuImage.InterfaceUUID) != region {
		return errors.Errorf("bitstream is not for this device: region(%s) and interface-uuid(%s) don't match", region, gbs.Metadata.AfuImage.InterfaceUUID)

	}

	if len(gbs.Metadata.AfuImage.AcceleratorClusters) != 1 {
		return errors.Errorf("%s/%s: 'accelerator-clusters' field not found", region, afu)
	}
	if canonize(gbs.Metadata.AfuImage.AcceleratorClusters[0].AcceleratorTypeUUID) != afu {
		return errors.Errorf("incorrect bitstream: AFU(%s) and accelerator-type-uuid(%s) don't match", afu, gbs.Metadata.AfuImage.AcceleratorClusters[0].AcceleratorTypeUUID)
	}

	return nil
}

func getFpgaConfArgs(dev string) ([]string, error) {
	realDevPath, err := findSysFsDevice(dev)
	if err != nil {
		return nil, err
	}
	if realDevPath == "" {
		return nil, nil
	}
	for p := realDevPath; strings.HasPrefix(p, "/sys/devices/pci"); p = filepath.Dir(p) {
		pciDevPath, err := filepath.EvalSymlinks(filepath.Join(p, "device"))
		if err != nil {
			continue
		}
		subs := pciAddressRE.FindStringSubmatch(filepath.Base(pciDevPath))
		if subs == nil || len(subs) != 4 {
			return nil, errors.Errorf("unable to parse PCI address %s", pciDevPath)
		}
		return []string{"-B", "0x" + subs[1], "-D", "0x" + subs[2], "-F", "0x" + subs[3]}, nil
	}
	return nil, errors.Errorf("can't find PCI device address for sysfs entry %s", realDevPath)
}

func findSysFsDevice(dev string) (string, error) {
	var devType string

	fi, err := os.Stat(dev)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", errors.Wrapf(err, "unable to get stat for %s", dev)
	}

	switch mode := fi.Mode(); {
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0:
		devType = "block"
	case mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0:
		devType = "char"
	default:
		return "", errors.Errorf("%s is not a device node", dev)
	}

	rdev := fi.Sys().(*syscall.Stat_t).Rdev
	major := unix.Major(rdev)
	minor := unix.Minor(rdev)
	if major == 0 {
		return "", errors.Errorf("%s is a virtual device node", dev)
	}
	devPath := fmt.Sprintf("/sys/dev/%s/%d:%d", devType, major, minor)
	realDevPath, err := filepath.EvalSymlinks(devPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed get realpath for %s", devPath)
	}
	return realDevPath, nil
}

func (bitStream *opaeBitStream) program() error {
	devNode := "/dev/intel-fpga-port." + bitStream.params.devNum
	args, err := getFpgaConfArgs(devNode)
	if err != nil {
		return errors.Wrapf(err, "failed get fpgaconf args for %s", devNode)
	}
	args = append(args, bitStream.path)
	output, err := bitStream.execer.Command(fpgaconf, args...).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to program AFU %s to socket %s, region %s: output: %s", bitStream.params.afu, bitStream.params.devNum, bitStream.params.region, string(output))
	}
	return nil
}

type openCLBitStream struct {
	path   string
	params fpgaParams
	execer utilsexec.Interface
}

func (bitStream *openCLBitStream) validate() error {
	return nil
}

func (bitStream *openCLBitStream) program() error {
	output, err := bitStream.execer.Command(aocl, "program", "acl"+bitStream.params.devNum, bitStream.path).CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "failed to program AFU %s to socket %s, region %s: output: %s", bitStream.params.afu, bitStream.params.devNum, bitStream.params.region, string(output))
	}

	return nil
}

func newHookEnv(bitStreamDir string, config string, execer utilsexec.Interface, afuIDTemplate string, interfaceIDTemplate string) *hookEnv {
	return &hookEnv{
		bitStreamDir,
		config,
		execer,
		afuIDTemplate,
		interfaceIDTemplate,
	}
}

func canonize(uuid string) string {
	return strings.ToLower(strings.Replace(uuid, "-", "", -1))
}

func (he *hookEnv) getFPGAParams(content map[string]interface{}) ([]fpgaParams, error) {
	bundle, ok := content["bundle"]
	if !ok {
		return nil, errors.New("no 'bundle' field in the configuration")
	}

	configPath := filepath.Join(fmt.Sprint(bundle), he.config)
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

	linux, ok := content["linux"]
	if !ok {
		return nil, errors.Errorf("no 'linux' field found in %s", configPath)
	}

	rawDevices, ok := linux.(map[string]interface{})["devices"]
	if !ok {
		return nil, errors.Errorf("no 'devices' field found in the 'linux' struct in %s", configPath)
	}

	regionEnv := make(map[string]string)
	afuEnv := make(map[string]string)
	envSet := make(map[string]struct{})
	for _, env := range rawEnv.([]interface{}) {
		splitted := strings.SplitN(env.(string), "=", 2)
		if strings.HasPrefix(splitted[0], fpgaRegionEnvPrefix) {
			regionEnv[splitted[0]] = splitted[1]
			underscoreSplitted := strings.SplitN(splitted[0], "_", 3)
			envSet[underscoreSplitted[2]] = struct{}{}
		} else if strings.HasPrefix(splitted[0], fpgaAfuEnvPrefix) {
			afuEnv[splitted[0]] = splitted[1]
			underscoreSplitted := strings.SplitN(splitted[0], "_", 3)
			envSet[underscoreSplitted[2]] = struct{}{}
		}
	}

	devSet := make(map[string]struct{})
	for _, device := range rawDevices.([]interface{}) {
		deviceNum := fpgaDevRE.FindStringSubmatch(device.(map[string]interface{})["path"].(string))
		if deviceNum != nil {
			devSet[deviceNum[1]] = struct{}{}
		}
	}

	return he.produceFPGAParams(regionEnv, afuEnv, envSet, devSet)
}

// produceFPGAParams produce FPGA params from parsed JSON data
func (he *hookEnv) produceFPGAParams(regionEnv, afuEnv map[string]string, envSet, devSet map[string]struct{}) ([]fpgaParams, error) {
	if len(envSet) == 0 {
		return nil, errors.Errorf("No correct FPGA environment variables set")
	}

	if len(envSet) != len(regionEnv) || len(envSet) != len(afuEnv) {
		return nil, errors.Errorf("Environment variables are set incorrectly")
	}

	if len(devSet) != len(envSet) {
		return nil, errors.Errorf("Environment variables don't correspond allocated devices")
	}

	params := []fpgaParams{}
	for bitstreamNum := range envSet {
		interfaceID := canonize(regionEnv[fpgaRegionEnvPrefix+bitstreamNum])
		found := false
		// Find a device suitable for the requested bitstream
		for devNum := range devSet {
			iID, err := he.getInterfaceID(devNum)
			if err != nil {
				return nil, err
			}

			if interfaceID == canonize(iID) {
				params = append(params,
					fpgaParams{
						afu:    canonize(afuEnv[fpgaAfuEnvPrefix+bitstreamNum]),
						region: interfaceID,
						devNum: devNum,
					},
				)
				delete(devSet, devNum)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("can't find appropriate device for interfaceID %s", interfaceID)
		}
	}

	return params, nil
}

func (he *hookEnv) getBitStream(params fpgaParams) (fpgaBitStream, error) {
	bitStreamPath := ""
	// Temporarily only support gbs bitstreams
	// for _, ext := range []string{".gbs", ".aocx"} {
	for _, ext := range []string{".gbs"} {
		bitStreamPath = filepath.Join(he.bitStreamDir, params.region, params.afu+ext)

		_, err := os.Stat(bitStreamPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, errors.Errorf("%s: stat error: %v", bitStreamPath, err)
		}

		if ext == ".gbs" {
			return &opaeBitStream{bitStreamPath, params, he.execer}, nil
		} else if ext == ".aocx" {
			return &openCLBitStream{bitStreamPath, params, he.execer}, nil
		}
	}
	return nil, errors.Errorf("%s/%s: bitstream not found", params.region, params.afu)
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

func (he *hookEnv) getInterfaceID(deviceNum string) (string, error) {
	// NOTE: only one region per device is supported, hence
	// deviceNum is used twice (device and FME numbers are the same)
	interfaceIDPath := fmt.Sprintf(he.interfaceIDTemplate, deviceNum, deviceNum)
	data, err := ioutil.ReadFile(interfaceIDPath)
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

	paramslist, err := he.getFPGAParams(content)
	if err != nil {
		return errors.WithMessage(err, "couldn't get FPGA region, AFU and device number")
	}

	for _, params := range paramslist {
		programmedAfu, err := he.getProgrammedAfu(params.devNum)
		if err != nil {
			return err
		}

		if canonize(programmedAfu) == params.afu {
			// Afu is already programmed
			return nil
		}

		bitStream, err := he.getBitStream(params)
		if err != nil {
			return err
		}

		err = bitStream.validate()
		if err != nil {
			return err
		}

		err = bitStream.program()
		if err != nil {
			return err
		}

		programmedAfu, err = he.getProgrammedAfu(params.devNum)
		if err != nil {
			return err
		}

		if programmedAfu != params.afu {
			return errors.Errorf("programmed function %s instead of %s", programmedAfu, params.afu)
		}
	}

	return nil
}

func main() {
	if os.Getenv("PATH") == "" { // runc doesn't set PATH when runs hooks
		os.Setenv("PATH", "/sbin:/usr/sbin:/usr/local/sbin:/usr/local/bin:/usr/bin:/bin")
	}

	he := newHookEnv(fpgaBitStreamDirectory, configJSON, utilsexec.New(), afuIDTemplate, interfaceIDTemplate)

	err := he.process(os.Stdin)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
}
