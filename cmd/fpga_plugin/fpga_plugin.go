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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	utilnode "k8s.io/kubernetes/pkg/util/node"

	"github.com/intel/intel-device-plugins-for-kubernetes/pkg/debug"
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
			regionTree.AddDevice(devType, region.id, dpapi.DeviceInfo{
				State: health,
				Nodes: devNodes,
			})
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
			regionTree.AddDevice(devType, region.id, dpapi.DeviceInfo{
				State: health,
				Nodes: devNodes,
			})
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
				devType := fmt.Sprintf("%s-%s", afMode, afu.afuID)
				afuTree.AddDevice(devType, afu.id, dpapi.DeviceInfo{
					State: health,
					Nodes: []pluginapi.DeviceSpec{
						{
							HostPath:      afu.devNode,
							ContainerPath: afu.devNode,
							Permissions:   "rw",
						},
					},
				})
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
}

// newDevicePlugin returns new instance of devicePlugin
func newDevicePlugin(mode string) (*devicePlugin, error) {
	if _, err := os.Stat(sysfsDirectoryOPAE); os.IsNotExist(err) {
		if _, err = os.Stat(sysfsDirectoryDFL); os.IsNotExist(err) {
			return nil, fmt.Errorf("kernel driver is not loaded: neither %s nor %s sysfs entry exists", sysfsDirectoryOPAE, sysfsDirectoryDFL)
		}
		return newDevicePluginDFL(sysfsDirectoryDFL, devfsDirectory, mode)
	}
	return newDevicePluginOPAE(sysfsDirectoryOPAE, devfsDirectory, mode)
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
	for {
		devTree, err := dp.scanFPGAs()
		if err != nil {
			return err
		}

		notifier.Notify(devTree)

		time.Sleep(5 * time.Second)
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

	debug.Print("Start new FPGA scan")

	fpgaFiles, err := ioutil.ReadDir(dp.sysfsDir)
	if err != nil {
		fmt.Printf("WARNING: Can't read folder %s. Kernel driver not loaded?\n", dp.sysfsDir)
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

func fatal(err error) {
	fmt.Printf("ERROR: %+v\n", err)
	os.Exit(1)
}

func main() {
	var mode string
	var kubeconfig string
	var master string
	var config *rest.Config
	var err error
	var debugEnabled bool

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.StringVar(&mode, "mode", string(afMode),
		fmt.Sprintf("device plugin mode: '%s' (default), '%s' or '%s'", afMode, regionMode, regionDevelMode))
	flag.BoolVar(&debugEnabled, "debug", false, "enable debug output")
	flag.Parse()

	if debugEnabled {
		debug.Activate()
	}

	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags(master, kubeconfig)
	}
	if err != nil {
		fatal(err)
	}

	// if NODE_NAME is not set then try to use hostname
	nodeName, err := utilnode.GetHostname(os.Getenv("NODE_NAME"))
	if err != nil {
		fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fatal(err)
	}

	node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		fatal(err)
	}

	if nodeMode, ok := node.ObjectMeta.Annotations["fpga.intel.com/device-plugin-mode"]; ok {
		fmt.Println("Overriding mode to ", nodeMode)
		mode = nodeMode
	}

	plugin, err := newDevicePlugin(mode)
	if err != nil {
		fatal(err)
	}

	fmt.Printf("FPGA device plugin (%s) started in %s mode\n", plugin.name, mode)

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
