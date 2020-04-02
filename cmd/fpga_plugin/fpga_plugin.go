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
	"strings"
	"time"

	"k8s.io/klog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin"
	"github.com/pkg/errors"
)

const (
	devfsDirectory     = "/dev"
	sysfsDirectoryOPAE = "/sys/class/fpga"
	sysfsDirectoryDFL  = "/sys/class/fpga_region"

	// Device plugin settings.
	namespace      = "fpga.intel.com"
	annotationName = "com.intel.fpga.mode"

	// Scanner's mode of operation
	afMode          = "af"
	regionMode      = "region"
	regionDevelMode = "regiondevel"

	// When the device's firmware crashes the driver reports these values
	unhealthyAfuID       = "ffffffffffffffffffffffffffffffff"
	unhealthyInterfaceID = "ffffffffffffffffffffffffffffffff"

	// Frequency of device scans
	scanFrequency = 5 * time.Second

	// Names of extended resources cannot be longer than 63 characters.
	// Therefore for AF resources we have to cut the interface ID prefix
	// to 31 characters only.
	interfaceIDPrefixLength = 31
)

type getDevTreeFunc func(devices []device) dpapi.DeviceTree
type getSysFsInfoFunc func(dp *devicePlugin, deviceFolder string, deviceFiles []os.FileInfo, fname string) ([]region, []afu, error)

// getRegionDevelTree returns mapping of region interface IDs to AF ports and FME devices
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

// getRegionTree returns mapping of region interface IDs to AF ports only
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

// getAfuTree returns mapping of AFU IDs to AF ports
func getAfuTree(devices []device) dpapi.DeviceTree {
	afuTree := dpapi.NewDeviceTree()

	for _, dev := range devices {
		for _, region := range dev.regions {
			for _, afu := range region.afus {
				health := pluginapi.Healthy
				if afu.afuID == unhealthyAfuID {
					health = pluginapi.Unhealthy
				}
				devType := region.interfaceID[:interfaceIDPrefixLength] + afu.afuID
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
	fmeReg    *regexp.Regexp

	getDevTree   getDevTreeFunc
	getSysFsInfo getSysFsInfoFunc

	ignoreAfuIDs       bool
	ignoreEmptyRegions bool
	annotationValue    string

	scanTicker *time.Ticker
	scanDone   chan bool
}

// newDevicePlugin returns new instance of devicePlugin
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

	dp.scanTicker = time.NewTicker(scanFrequency)
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

// Scan starts scanning FPGA devices on the host
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

func (dp *devicePlugin) getDevNode(devName string) (string, error) {
	devNode := path.Join(dp.devfsDir, devName)
	if _, err := os.Stat(devNode); err != nil {
		return "", errors.Wrapf(err, "Device %s doesn't exist", devNode)
	}

	return devNode, nil
}

func (dp *devicePlugin) getAFU(fpath string, devName string) (*afu, error) {
	var afuID string

	if dp.ignoreAfuIDs {
		afuID = "unused_afu_id"
	} else {
		data, err := ioutil.ReadFile(fpath)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		afuID = strings.TrimSpace(string(data))
	}

	devNode, err := dp.getDevNode(devName)
	if err != nil {
		return nil, err
	}
	return &afu{
		id:      devName,
		afuID:   afuID,
		devNode: devNode,
	}, nil
}

func (dp *devicePlugin) getFME(interfaceIDPath string, devName string) (*region, error) {
	data, err := ioutil.ReadFile(interfaceIDPath)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	devNode, err := dp.getDevNode(devName)
	if err != nil {
		return nil, err
	}

	return &region{
		id:          devName,
		interfaceID: strings.TrimSpace(string(data)),
		devNode:     devNode,
	}, nil
}

func (dp *devicePlugin) scanFPGAs() (dpapi.DeviceTree, error) {
	var devices []device

	klog.V(4).Info("Start new FPGA scan")

	fpgaFiles, err := ioutil.ReadDir(dp.sysfsDir)
	if err != nil {
		klog.Warningf("Can't read folder %s. Kernel driver not loaded?", dp.sysfsDir)
		return dp.getDevTree([]device{}), nil
	}

	for _, fpgaFile := range fpgaFiles {
		fname := fpgaFile.Name()

		if !dp.deviceReg.MatchString(fname) {
			continue
		}

		deviceFolder := path.Join(dp.sysfsDir, fname)
		deviceFiles, err := ioutil.ReadDir(deviceFolder)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		regions, afus, err := dp.getSysFsInfo(dp, deviceFolder, deviceFiles, fname)
		if err != nil {
			return nil, err
		}

		if len(regions) == 0 {
			if len(afus) > 0 {
				return nil, errors.Errorf("%s: AFU without corresponding FME found", fname)
			}
			if dp.ignoreEmptyRegions {
				continue // not a base DFL region, skipping
			}
			return nil, errors.Errorf("%s: No regions found", fname)
		}

		// Currently only one region per device is supported.
		regions[0].afus = afus
		devices = append(devices, device{
			name:    fname,
			regions: regions,
		})
	}

	return dp.getDevTree(devices), nil
}

// getPluginParams is a helper function to avoid code duplication
// it's used in newDevicePluginOPAE and newDevicePluginDFL
func getPluginParams(mode string) (getDevTreeFunc, bool, string, error) {
	var getDevTree getDevTreeFunc

	ignoreAfuIDs := false
	annotationValue := ""

	switch mode {
	case afMode:
		getDevTree = getAfuTree
	case regionMode:
		getDevTree = getRegionTree
		ignoreAfuIDs = true
		annotationValue = fmt.Sprintf("%s/%s", namespace, regionMode)
	case regionDevelMode:
		getDevTree = getRegionDevelTree
		ignoreAfuIDs = true
	default:
		return nil, ignoreAfuIDs, annotationValue, errors.Errorf("Wrong mode: '%s'", mode)
	}

	return getDevTree, ignoreAfuIDs, annotationValue, nil
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
