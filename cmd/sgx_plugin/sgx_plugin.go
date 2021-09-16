// Copyright 2020 Intel Corporation. All Rights Reserved.
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
	"os"
	"path"
	"runtime"
	"strconv"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"k8s.io/klog/v2"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	// Device plugin settings.
	namespace                   = "sgx.intel.com"
	deviceTypeEnclave           = "enclave"
	deviceTypeProvision         = "provision"
	devicePath                  = "/dev"
	podsPerCoreEnvVariable      = "PODS_PER_CORE"
	defaultPodCount        uint = 110
)

type devicePlugin struct {
	scanDone   chan bool
	devfsDir   string
	nEnclave   uint
	nProvision uint
}

func newDevicePlugin(devfsDir string, nEnclave, nProvision uint) *devicePlugin {
	return &devicePlugin{
		devfsDir:   devfsDir,
		nEnclave:   nEnclave,
		nProvision: nProvision,
		scanDone:   make(chan bool, 1),
	}
}

func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	devTree, err := dp.scan()
	if err != nil {
		return err
	}
	notifier.Notify(devTree)

	// Wait forever to prevent manager run loop from exiting.
	<-dp.scanDone
	return nil
}

func (dp *devicePlugin) scan() (dpapi.DeviceTree, error) {
	devTree := dpapi.NewDeviceTree()

	// Assume that both /dev/sgx_enclave and /dev/sgx_provision must be present.
	sgxEnclavePath := path.Join(dp.devfsDir, "sgx_enclave")
	sgxProvisionPath := path.Join(dp.devfsDir, "sgx_provision")
	if _, err := os.Stat(sgxEnclavePath); err != nil {
		klog.Error("No SGX enclave file available: ", err)
		return devTree, nil
	}
	if _, err := os.Stat(sgxProvisionPath); err != nil {
		klog.Error("No SGX provision file available: ", err)
		return devTree, nil
	}

	deprecatedMounts := []pluginapi.Mount{
		{
			HostPath:      "/dev/sgx",
			ContainerPath: "/dev/sgx",
		},
	}

	for i := uint(0); i < dp.nEnclave; i++ {
		devID := fmt.Sprintf("%s-%d", "sgx-enclave", i)
		nodes := []pluginapi.DeviceSpec{{HostPath: sgxEnclavePath, ContainerPath: sgxEnclavePath, Permissions: "rw"}}
		devTree.AddDevice(deviceTypeEnclave, devID, dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, deprecatedMounts, nil))
	}
	for i := uint(0); i < dp.nProvision; i++ {
		devID := fmt.Sprintf("%s-%d", "sgx-provision", i)
		nodes := []pluginapi.DeviceSpec{{HostPath: sgxProvisionPath, ContainerPath: sgxProvisionPath, Permissions: "rw"}}
		devTree.AddDevice(deviceTypeProvision, devID, dpapi.NewDeviceInfo(pluginapi.Healthy, nodes, deprecatedMounts, nil))
	}
	return devTree, nil
}

func getDefaultPodCount(nCPUs uint) uint {
	// By default we provide as many enclave resources as there can be pods
	// running on the node. The problem is that this value is configurable
	// either via "--pods-per-core" or "--max-pods" kubelet options. We get the
	// limit by multiplying the number of cores in the system with env variable
	// "PODS_PER_CORE".

	envPodsPerCore := os.Getenv(podsPerCoreEnvVariable)
	if envPodsPerCore != "" {
		tmp, err := strconv.ParseUint(envPodsPerCore, 10, 32)
		if err != nil {
			klog.Errorf("Error: failed to parse %s value as uint, using default value.", podsPerCoreEnvVariable)
		} else {
			return uint(tmp) * nCPUs
		}
	}

	return defaultPodCount
}

func main() {
	var enclaveLimit uint
	var provisionLimit uint

	podCount := getDefaultPodCount(uint(runtime.NumCPU()))

	flag.UintVar(&enclaveLimit, "enclave-limit", podCount, "Number of \"enclave\" resources")
	flag.UintVar(&provisionLimit, "provision-limit", podCount, "Number of \"provision\" resources")
	flag.Parse()

	klog.V(4).Infof("SGX device plugin started with %d \"%s/enclave\" resources and %d \"%s/provision\" resources.", enclaveLimit, namespace, provisionLimit, namespace)

	plugin := newDevicePlugin(devicePath, enclaveLimit, provisionLimit)
	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
