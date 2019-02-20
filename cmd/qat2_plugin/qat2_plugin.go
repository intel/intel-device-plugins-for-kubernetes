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
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	utilsexec "k8s.io/utils/exec"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/ini"
)

const (
	namespace = "qat.intel.com"
)

var (
	uioRegex = regexp.MustCompile(`^uio[0-9]+$`)
)

type endpoint struct {
	id        string
	processes int
}

type section struct {
	endpoints          []endpoint
	cryptoEngines      int
	compressionEngines int
	pinned             bool
}

func newDeviceSpec(devPath string) pluginapi.DeviceSpec {
	return pluginapi.DeviceSpec{
		HostPath:      devPath,
		ContainerPath: devPath,
		Permissions:   "rw",
	}
}

func getDevTree(devfs string, config map[string]section) (dpapi.DeviceTree, error) {
	devTree := dpapi.NewDeviceTree()

	devFiles, err := ioutil.ReadDir(devfs)
	if err != nil {
		return devTree, errors.Wrapf(err, "Can't read %s", devfs)
	}

	devs := []pluginapi.DeviceSpec{
		newDeviceSpec(path.Join(devfs, "qat_adf_ctl")),
		newDeviceSpec(path.Join(devfs, "qat_dev_processes")),
		newDeviceSpec(path.Join(devfs, "usdm_drv")),
	}
	for _, devFile := range devFiles {
		fname := devFile.Name()

		if uioRegex.MatchString(fname) {
			devs = append(devs, newDeviceSpec(path.Join(devfs, fname)))
		}
	}

	uniqID := 0
	for sname, svalue := range config {
		var devType string

		devType = fmt.Sprintf("cy%d_dc%d", svalue.cryptoEngines, svalue.compressionEngines)
		for _, ep := range svalue.endpoints {
			for i := 0; i < ep.processes; i++ {
				devTree.AddDevice(devType, fmt.Sprintf("%s_%s_%d", sname, ep.id, i), dpapi.DeviceInfo{
					State: pluginapi.Healthy,
					Nodes: devs,
					Envs: map[string]string{
						fmt.Sprintf("QAT_SECTION_NAME_%s_%d", devType, uniqID): sname,
						// This env variable may get overriden if a container requests more than one QAT process.
						// But we keep this code since the majority of pod workloads run only one QAT process.
						// The rest should use QAT_SECTION_NAME_XXX variables.
						"QAT_SECTION_NAME": sname,
					},
				})
				uniqID++
			}

			if !svalue.pinned {
				break
			}
		}
	}

	return devTree, nil
}

type devicePlugin struct {
	execer    utilsexec.Interface
	configDir string
}

func newDevicePlugin(configDir string, execer utilsexec.Interface) *devicePlugin {
	return &devicePlugin{
		execer:    execer,
		configDir: configDir,
	}
}

func (dp *devicePlugin) parseConfigs() (map[string]section, error) {
	outputBytes, err := dp.execer.Command("adf_ctl", "status").CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "Can't get driver status")
	}
	output := string(outputBytes[:])

	devNum := 0
	driverConfig := make(map[string]section)
	for ln, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, " qat_") {
			continue
		}

		devstr := strings.SplitN(line, "-", 2)
		if len(devstr) != 2 {
			continue
		}

		devprops := strings.Split(devstr[1], ",")
		devType := ""
		for _, propstr := range devprops {
			switch strings.TrimSpace(propstr) {
			// Embeded in Chipset c62x.
			case "type: c6xx":
				devType = "c6xx"
			// Cards with communication chipset 8925-8955.
			case "type: dh895xcc":
				devType = "dh895xcc"
			}
		}

		if devType == "" {
			continue
		}

		devID := strings.TrimPrefix(strings.TrimSpace(devstr[0]), "qat_")

		f, err := os.Open(path.Join(dp.configDir, fmt.Sprintf("%s_%s.conf", devType, devID)))
		if err != nil {
			return nil, errors.WithStack(err)
		}
		defer f.Close()

		// Parse the configuration.
		config, err := ini.Parse(f)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		devNum++

		debug.Print(ln, devID, line)

		for sectionName, data := range config {
			if sectionName == "GENERAL" || sectionName == "KERNEL" || sectionName == "KERNEL_QAT" || sectionName == "" {
				continue
			}
			debug.Print(sectionName)

			numProcesses, err := strconv.Atoi(data["NumProcesses"])
			if err != nil {
				return nil, errors.Wrapf(err, "Can't convert NumProcesses in %s", sectionName)
			}
			cryptoEngines, err := strconv.Atoi(data["NumberCyInstances"])
			if err != nil {
				return nil, errors.Wrapf(err, "Can't convert NumberCyInstances in %s", sectionName)
			}
			compressionEngines, err := strconv.Atoi(data["NumberDcInstances"])
			if err != nil {
				return nil, errors.Wrapf(err, "Can't convert NumberDcInstances in %s", sectionName)
			}
			pinned := false
			if limitDevAccess, ok := data["LimitDevAccess"]; ok {
				if limitDevAccess != "0" {
					pinned = true
				}
			}

			if old, ok := driverConfig[sectionName]; ok {
				// first check the sections are consistent across endpoints
				if old.pinned != pinned {
					return nil, errors.Errorf("Value of LimitDevAccess must be consistent across all devices in %s", sectionName)
				}
				if !pinned && old.endpoints[0].processes != numProcesses {
					return nil, errors.Errorf("For not pinned section \"%s\" NumProcesses must be equal for all devices", sectionName)
				}
				if old.cryptoEngines != cryptoEngines || old.compressionEngines != compressionEngines {
					return nil, errors.Errorf("NumberCyInstances and NumberDcInstances must be consistent across all devices in %s", sectionName)
				}

				// then add a new endpoint to the section
				old.endpoints = append(old.endpoints, endpoint{
					id:        devID,
					processes: numProcesses,
				})
				driverConfig[sectionName] = old
			} else {
				driverConfig[sectionName] = section{
					endpoints: []endpoint{
						{
							id:        devID,
							processes: numProcesses,
						},
					},
					cryptoEngines:      cryptoEngines,
					compressionEngines: compressionEngines,
					pinned:             pinned,
				}
			}
		}

	}

	// check if the number of sections with LimitDevAccess=1 is equal to the number of endpoints
	for sname, svalue := range driverConfig {
		if svalue.pinned && len(svalue.endpoints) != devNum {
			return nil, errors.Errorf("Section [%s] must be defined for all QAT devices since it contains LimitDevAccess=1", sname)
		}
	}

	return driverConfig, nil
}

func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	for {
		driverConfig, err := dp.parseConfigs()
		if err != nil {
			return err
		}

		devTree, err := getDevTree("/dev", driverConfig)
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		time.Sleep(5 * time.Second)
	}
}

func (dp *devicePlugin) PostAllocate(response *pluginapi.AllocateResponse) error {
	for _, containerResponse := range response.GetContainerResponses() {
		envsToDelete := []string{}
		envsToAdd := make(map[string]string)
		counter := 0
		for key, value := range containerResponse.Envs {
			if !strings.HasPrefix(key, "QAT_SECTION_NAME_") {
				continue
			}
			parts := strings.Split(key, "_")
			if len(parts) != 6 {
				return errors.Errorf("Wrong format of env variable name %s", key)
			}
			prefix := strings.Join(parts[0:5], "_")
			envsToDelete = append(envsToDelete, key)
			envsToAdd[fmt.Sprintf("%s_%d", prefix, counter)] = value
			counter++
		}

		for _, key := range envsToDelete {
			delete(containerResponse.Envs, key)
		}

		for key, value := range envsToAdd {
			containerResponse.Envs[key] = value
		}
	}

	return nil
}

func main() {
	debugEnabled := flag.Bool("debug", false, "enable debug output")
	flag.Parse()

	if *debugEnabled {
		debug.Activate()
	}

	plugin := newDevicePlugin("/etc", utilsexec.New())

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
