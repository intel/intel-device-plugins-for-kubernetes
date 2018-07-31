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

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	utilnode "k8s.io/kubernetes/pkg/util/node"

	dpapi "github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

const (
	sysfsDirectory = "/sys/class/fpga"
	devfsDirectory = "/dev"

	// Device plugin settings.
	namespace      = "fpga.intel.com"
	annotationName = "com.intel.fpga.mode"

	// Scanner's mode of operation
	afMode          = "af"
	regionMode      = "region"
	regionDevelMode = "regiondevel"

	deviceRE = `^intel-fpga-dev.[0-9]+$`
	portRE   = `^intel-fpga-port.[0-9]+$`
	fmeRE    = `^intel-fpga-fme.[0-9]+$`
)

type getDevTreeFunc func(devices []device) dpapi.DeviceTree

// getRegionDevelTree returns mapping of region interface IDs to AF ports and FME devices
func getRegionDevelTree(devices []device) dpapi.DeviceTree {
	regionTree := dpapi.NewDeviceTree()

	for _, dev := range devices {
		for _, region := range dev.regions {
			devType := fmt.Sprintf("%s-%s", regionMode, region.interfaceID)
			devNodes := make([]string, len(region.afus)+1)
			for num, afu := range region.afus {
				devNodes[num] = afu.devNode
			}
			devNodes[len(region.afus)] = region.devNode
			regionTree.AddDevice(devType, region.id, dpapi.DeviceInfo{
				State: pluginapi.Healthy,
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
			devType := fmt.Sprintf("%s-%s", regionMode, region.interfaceID)
			devNodes := make([]string, len(region.afus))
			for num, afu := range region.afus {
				devNodes[num] = afu.devNode
			}
			regionTree.AddDevice(devType, region.id, dpapi.DeviceInfo{
				State: pluginapi.Healthy,
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
				devType := fmt.Sprintf("%s-%s", afMode, afu.afuID)
				afuTree.AddDevice(devType, afu.id, dpapi.DeviceInfo{
					State: pluginapi.Healthy,
					Nodes: []string{afu.devNode},
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
	sysfsDir string
	devfsDir string

	deviceReg *regexp.Regexp
	portReg   *regexp.Regexp
	fmeReg    *regexp.Regexp

	getDevTree      getDevTreeFunc
	ignoreAfuIDs    bool
	annotationValue string
}

// newDevicePlugin returns new instance of devicePlugin
func newDevicePlugin(sysfsDir string, devfsDir string, mode string) (*devicePlugin, error) {
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
		return nil, fmt.Errorf("Wrong mode: '%s'", mode)
	}

	return &devicePlugin{
		sysfsDir:        sysfsDir,
		devfsDir:        devfsDir,
		deviceReg:       regexp.MustCompile(deviceRE),
		portReg:         regexp.MustCompile(portRE),
		fmeReg:          regexp.MustCompile(fmeRE),
		getDevTree:      getDevTree,
		ignoreAfuIDs:    ignoreAfuIDs,
		annotationValue: annotationValue,
	}, nil
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

// Scan starts scanning of FPGA devices on the host
func (dp *devicePlugin) Scan(notifier dpapi.Notifier) error {
	for {
		devTree, err := dp.scanFPGAs()
		if err != nil {
			glog.Error("Device scan failed: ", err)
			return fmt.Errorf("Device scan failed: %v", err)
		}

		notifier.Notify(devTree)

		time.Sleep(5 * time.Second)
	}
}

func (dp *devicePlugin) getDevNode(devName string) (string, error) {
	devNode := path.Join(dp.devfsDir, devName)
	if _, err := os.Stat(devNode); err != nil {
		return "", fmt.Errorf("Device %s doesn't exist: %v", devNode, err)
	}

	return devNode, nil
}

func (dp *devicePlugin) scanFPGAs() (dpapi.DeviceTree, error) {
	var devices []device

	glog.V(2).Info("Start new FPGA scan")

	fpgaFiles, err := ioutil.ReadDir(dp.sysfsDir)
	if err != nil {
		return nil, fmt.Errorf("Can't read sysfs folder: %v", err)
	}

	for _, fpgaFile := range fpgaFiles {
		fname := fpgaFile.Name()

		if !dp.deviceReg.MatchString(fname) {
			continue
		}

		deviceFolder := path.Join(dp.sysfsDir, fname)
		deviceFiles, err := ioutil.ReadDir(deviceFolder)
		if err != nil {
			return nil, err
		}

		regions, afus, err := dp.getSysFsInfo(deviceFolder, deviceFiles, fname)
		if err != nil {
			return nil, err
		}

		if len(regions) == 0 {
			return nil, fmt.Errorf("No regions found for device %s", fname)
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

func (dp *devicePlugin) getSysFsInfo(deviceFolder string, deviceFiles []os.FileInfo, fname string) ([]region, []afu, error) {
	var regions []region
	var afus []afu
	for _, deviceFile := range deviceFiles {
		name := deviceFile.Name()

		if dp.fmeReg.MatchString(name) {
			if len(regions) > 0 {
				return nil, nil, fmt.Errorf("Detected more than one FPGA region for device %s. Only one region per FPGA device is supported", fname)
			}
			interfaceIDFile := path.Join(deviceFolder, name, "pr", "interface_id")
			data, err := ioutil.ReadFile(interfaceIDFile)
			if err != nil {
				return nil, nil, err
			}
			devNode, err := dp.getDevNode(name)
			if err != nil {
				return nil, nil, err
			}
			regions = append(regions, region{
				id:          name,
				interfaceID: strings.TrimSpace(string(data)),
				devNode:     devNode,
			})
		} else if dp.portReg.MatchString(name) {
			var afuID string

			if dp.ignoreAfuIDs {
				afuID = "unused_afu_id"
			} else {
				afuFile := path.Join(deviceFolder, name, "afu_id")
				data, err := ioutil.ReadFile(afuFile)
				if err != nil {
					return nil, nil, err
				}
				afuID = strings.TrimSpace(string(data))
			}
			devNode, err := dp.getDevNode(name)
			if err != nil {
				return nil, nil, err
			}
			afus = append(afus, afu{
				id:      name,
				afuID:   afuID,
				devNode: devNode,
			})
		}
	}

	return regions, afus, nil
}

func main() {
	var mode string
	var kubeconfig string
	var master string
	var config *rest.Config
	var err error

	flag.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.StringVar(&master, "master", "", "master url")
	flag.StringVar(&mode, "mode", string(afMode),
		fmt.Sprintf("device plugin mode: '%s' (default), '%s' or '%s'", afMode, regionMode, regionDevelMode))
	flag.Parse()

	if kubeconfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags(master, kubeconfig)
	}
	if err != nil {
		glog.Fatal(err)
	}

	// if NODE_NAME is not set then try to use hostname
	nodeName := utilnode.GetHostname(os.Getenv("NODE_NAME"))

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}

	node, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		glog.Fatal(err)
	}

	if nodeMode, ok := node.ObjectMeta.Annotations["fpga.intel.com/device-plugin-mode"]; ok {
		glog.Info("Overriding mode to ", nodeMode)
		mode = nodeMode
	}

	plugin, err := newDevicePlugin(sysfsDirectory, devfsDirectory, mode)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}

	glog.Info("FPGA device plugin started in ", mode, " mode")

	manager := dpapi.NewManager(namespace, plugin)
	manager.Run()
}
