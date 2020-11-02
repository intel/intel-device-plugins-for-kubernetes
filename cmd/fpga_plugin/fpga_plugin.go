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
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"time"

	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/fpga"
	"github.com/pkg/errors"
)

const (
	devfsDirectory     = "/dev"
	sysfsDirectoryOPAE = "/sys/class/fpga"
	sysfsDirectoryDFL  = "/sys/class/fpga_region"

	// Device plugin settings.
	namespace      = "fpga.intel.com"
	annotationName = "com.intel.fpga.mode"

	// Scanner's mode of operation.
	afMode          = "af"
	regionMode      = "region"
	regionDevelMode = "regiondevel"

	// When the device's firmware crashes the driver reports these values.
	unhealthyAfuID       = "ffffffffffffffffffffffffffffffff"
	unhealthyInterfaceID = "ffffffffffffffffffffffffffffffff"

	// Period of device scans.
	scanPeriod = 5 * time.Second
)

type newPortFunc func(fname string) (fpga.Port, error)
type getDevTreeFunc func(devices []device) dpapi.DeviceTree

// getRegionDevelTree returns mapping of region interface IDs to AF ports and FME devices.
func getRegionDevelTree(devices []device) dpapi.DeviceTree {
	regionTree := dpapi.NewDeviceTree()

	for _, dev := range devices {
		for _, region := range dev.regions {
			health := pluginapi.Healthy
			if region.interfaceID == unhealthyInterfaceID {
				health = pluginapi.Unhealthy
			}
			devType := fmt.Sprintf("%s-%s", regionMode, region.interfaceID)
			devNodes := make([]pluginapi.DeviceSpec, len(region.afus)+1)
			for num, afu := range region.afus {
				devNodes[num] = pluginapi.DeviceSpec{
					HostPath:      afu.devNode,
					ContainerPath: afu.devNode,
					Permissions:   "rw",
				}
			}
			devNodes[len(region.afus)] = pluginapi.DeviceSpec{
				HostPath:      region.devNode,
				ContainerPath: region.devNode,
				Permissions:   "rw",
			}

			regionTree.AddDevice(devType, region.id, dpapi.NewDeviceInfo(health, devNodes, nil, nil))
		}
	}

	return regionTree
}

// getRegionTree returns mapping of region interface IDs to AF ports only.
func getRegionTree(devices []device) dpapi.DeviceTree {
	regionTree := dpapi.NewDeviceTree()

	for _, dev := range devices {
		for _, region := range dev.regions {
			health := pluginapi.Healthy
			if region.interfaceID == unhealthyInterfaceID {
				health = pluginapi.Unhealthy
			}
			devType := fmt.Sprintf("%s-%s", regionMode, region.interfaceID)
			devNodes := make([]pluginapi.DeviceSpec, len(region.afus))
			for num, afu := range region.afus {
				devNodes[num] = pluginapi.DeviceSpec{
					HostPath:      afu.devNode,
					ContainerPath: afu.devNode,
					Permissions:   "rw",
				}
			}
			regionTree.AddDevice(devType, region.id, dpapi.NewDeviceInfo(health, devNodes, nil, nil))
		}
	}

	return regionTree
}

// getAfuTree returns mapping of AFU IDs to AF ports.
func getAfuTree(devices []device) dpapi.DeviceTree {
	afuTree := dpapi.NewDeviceTree()

	for _, dev := range devices {
		for _, region := range dev.regions {
			for _, afu := range region.afus {
				health := pluginapi.Healthy
				if afu.afuID == unhealthyAfuID {
					health = pluginapi.Unhealthy
				}
				devType, err := fpga.GetAfuDevType(region.interfaceID, afu.afuID)
				if err != nil {
					klog.Warningf("failed to get devtype: %+v", err)
					continue
				}
				devNodes := []pluginapi.DeviceSpec{
					{
						HostPath:      afu.devNode,
						ContainerPath: afu.devNode,
						Permissions:   "rw",
					},
				}
				afuTree.AddDevice(devType, afu.id, dpapi.NewDeviceInfo(health, devNodes, nil, nil))
			}
		}
	}

	return afuTree
}

type afu struct {
	id      string
	afuID   string
	devNode string
}

type region struct {
	id          string
	interfaceID string
	devNode     string
	afus        []afu
}

type device struct {
	name    string
	regions []region
}

type devicePlugin struct {
	name string

	sysfsDir string
	devfsDir string

	deviceReg *regexp.Regexp
	portReg   *regexp.Regexp

	getDevTree getDevTreeFunc
	newPort    newPortFunc

	annotationValue string

	scanTicker *time.Ticker
	scanDone   chan bool
}

// newDevicePlugin returns new instance of devicePlugin.
func newDevicePlugin(mode string, rootPath string) (*devicePlugin, error) {
	var dp *devicePlugin
	var err error

	sysfsPathOPAE := path.Join(rootPath, sysfsDirectoryOPAE)
	devfsPath := path.Join(rootPath, devfsDirectory)
	if _, err = os.Stat(sysfsPathOPAE); os.IsNotExist(err) {
		sysfsPathDFL := path.Join(rootPath, sysfsDirectoryDFL)
		if _, err = os.Stat(sysfsPathDFL); os.IsNotExist(err) {
			return nil, fmt.Errorf("kernel driver is not loaded: neither %s nor %s sysfs entry exists", sysfsPathOPAE, sysfsPathDFL)
		}
		dp, err = newDevicePluginDFL(sysfsPathDFL, devfsPath, mode)
	} else {
		dp, err = newDevicePluginOPAE(sysfsPathOPAE, devfsPath, mode)
	}

	if err != nil {
		return nil, err
	}

	dp.newPort = fpga.NewPort
	dp.scanTicker = time.NewTicker(scanPeriod)
	dp.scanDone = make(chan bool, 1) // buffered as we may send to it before Scan starts receiving from it

	return dp, nil
}

func (dp *devicePlugin) PostAllocate(response *pluginapi.AllocateResponse) error {
	// Set container annotations when programming is allowed
	if len(dp.annotationValue) > 0 {
		for _, containerResponse := range response.GetContainerResponses() {
			containerResponse.Annotations = map[string]string{
				annotationName: dp.annotationValue,
			}
		}
	}

	return nil
}

// Scan starts scanning FPGA devices on the host.
func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	defer dp.scanTicker.Stop()
	for {
		devTree, err := dp.scanFPGAs()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		select {
		case <-dp.scanDone:
			return nil
		case <-dp.scanTicker.C:
		}
	}
}

func (dp *devicePlugin) getRegions(deviceFiles []os.FileInfo) ([]region, error) {
	regions := map[string]region{}
	for _, deviceFile := range deviceFiles {
		name := deviceFile.Name()
		if dp.portReg.MatchString(name) {
			port, err := dp.newPort(name)
			if err != nil {
				return nil, errors.Wrapf(err, "can't get port info for %s", name)
			}
			fme, err := port.GetFME()
			if err != nil {
				return nil, errors.Wrapf(err, "can't get FME info for %s", name)
			}

			afuInfo := afu{id: port.GetName(), afuID: port.GetAcceleratorTypeUUID(), devNode: port.GetDevPath()}
			regionName := fme.GetName()
			reg, ok := regions[regionName]
			if ok {
				reg.afus = append(reg.afus, afuInfo)
			} else {
				regions[regionName] = region{id: regionName, interfaceID: fme.GetInterfaceUUID(), devNode: fme.GetDevPath(), afus: []afu{afuInfo}}
			}
		}
	}
	result := make([]region, 0, len(regions))
	// Get list of regions from the map
	for _, reg := range regions {
		result = append(result, reg)
	}
	return result, nil
}

func (dp *devicePlugin) scanFPGAs() (dpapi.DeviceTree, error) {
	files, err := ioutil.ReadDir(dp.sysfsDir)
	if err != nil {
		klog.Warningf("Can't read folder %s. Kernel driver not loaded?", dp.sysfsDir)
		return dp.getDevTree([]device{}), nil
	}

	devices := []device{}
	for _, file := range files {
		devName := file.Name()

		if !dp.deviceReg.MatchString(devName) {
			continue
		}

		deviceFiles, err := ioutil.ReadDir(path.Join(dp.sysfsDir, devName))
		if err != nil {
			return nil, errors.WithStack(err)
		}

		regions, err := dp.getRegions(deviceFiles)
		if err != nil {
			return nil, err
		}

		if len(regions) > 0 {
			devices = append(devices, device{name: devName, regions: regions})
		}
	}
	return dp.getDevTree(devices), nil
}

// getPluginParams is a helper function to avoid code duplication.
// It's used in newDevicePluginOPAE and newDevicePluginDFL.
func getPluginParams(mode string) (getDevTreeFunc, string, error) {
	var getDevTree getDevTreeFunc

	annotationValue := ""

	switch mode {
	case afMode:
		getDevTree = getAfuTree
	case regionMode:
		getDevTree = getRegionTree
		annotationValue = fmt.Sprintf("%s/%s", namespace, regionMode)
	case regionDevelMode:
		getDevTree = getRegionDevelTree
	default:
		return nil, annotationValue, errors.Errorf("Wrong mode: '%s'", mode)
	}

	return getDevTree, annotationValue, nil
}

func main() {
	var mode string
	var kubeconfig string
	var master string
	var nodename string

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.StringVar(&nodename, "node-name", os.Getenv("NODE_NAME"), "node name in the cluster to query mode annotation from")
	flag.StringVar(&mode, "mode", string(afMode),
		fmt.Sprintf("device plugin mode: '%s' (default), '%s' or '%s'", afMode, regionMode, regionDevelMode))
	flag.Parse()

	nodeMode, err := getModeOverrideFromCluster(nodename, kubeconfig, master, mode)
	if err != nil {
		klog.Warningf("could not get mode override from cluster: %+v", err)
	}

	var modeMessage string
	if mode != nodeMode {
		modeMessage = fmt.Sprintf(" (override from %s node annotation)", nodename)
		mode = nodeMode
	} else {
		modeMessage = ""
	}

	plugin, err := newDevicePlugin(mode, "")
	if err != nil {
		klog.Fatalf("%+v", err)
	}

	klog.V(1).Infof("FPGA device plugin (%s) started in %s mode%s", plugin.name, mode, modeMessage)
	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
