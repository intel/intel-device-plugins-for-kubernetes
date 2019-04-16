// Copyright 2017 Intel Corporation. All Rights Reserved.
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
	"github.com/pkg/errors"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	"time"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
)

const (
	iommuGroupDirectory = "/sys/kernel/iommu_groups"
	vfioDevicePath      = "/dev/vfio"
	iommuGroupDevices   = "devices"
	uuidPattern         = "[0-9a-z]{8}-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{4}-[0-9a-z]{12}"

	// Device plugin settings.
	namespace  = "mdev.intel.com"
	deviceType = "mdev"
)

type devicePlugin struct {
	iommuGroupDir string
	vfioDevPath   string
}

func newDevicePlugin(iommuGroupDir string, vfioDevPath string) *devicePlugin {
	return &devicePlugin{
		iommuGroupDir: iommuGroupDir,
		vfioDevPath:   vfioDevPath,
	}
}

func (dp *devicePlugin) Scan(notifier deviceplugin.Notifier) error {
	for {
		devTree, err := dp.scan()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		time.Sleep(5 * time.Second)
	}
}

func (dp *devicePlugin) scan() (deviceplugin.DeviceTree, error) {
	vfioNodes, err := ioutil.ReadDir(dp.vfioDevPath)

	if err != nil {
		fmt.Println("Can't read vfio Device Path: ", dp.vfioDevPath)
	}
	devTree := deviceplugin.NewDeviceTree()
	for _, node := range vfioNodes {
		if strings.HasSuffix(node.Name(), "vfio") {
			continue
		}
		iommuGroupNode := path.Join(dp.iommuGroupDir, node.Name())

		vfioDevices, err := ioutil.ReadDir(path.Join(iommuGroupNode, iommuGroupDevices))

		if err != nil {
			return nil, errors.Wrapf(err, "Can't read %s vfio device", vfioDevices)
		}
		for _, device := range vfioDevices {

			match, _ := regexp.MatchString(uuidPattern, device.Name()) // Judge whether it is MDEV decice

			if match {
				var nodes []pluginapi.DeviceSpec

				devDir := path.Join(iommuGroupNode, device.Name())

				mdevNode := path.Join(dp.vfioDevPath, node.Name())

				fmt.Printf("The %s device detected, the corresponding MDEV device detected is %s\n", devDir, mdevNode)

				nodes = append(nodes, pluginapi.DeviceSpec{
					HostPath:      mdevNode,
					ContainerPath: mdevNode,
					Permissions:   "rw",
				})

				devInfo := deviceplugin.DeviceInfo{
					State: pluginapi.Healthy,
					Nodes: nodes,
				}
				devTree.AddDevice("mdev", devDir, devInfo)
			}
		}
	}

	return devTree, nil
}

func main() {
	debugEnabled := flag.Bool("debug", false, "enable debug output")
	flag.Parse()
	fmt.Println("MDEV device plugin started")

	if *debugEnabled {
		debug.Activate()
	}

	plugin := newDevicePlugin(iommuGroupDirectory, vfioDevicePath)
	manager := deviceplugin.NewManager(namespace, plugin)
	manager.Run()
}
