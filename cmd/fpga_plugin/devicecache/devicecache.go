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

package devicecache

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

// Device Cache's mode of operation
const (
	AfMode     = "af"
	RegionMode = "region"
)

const (
	sysfsDirectory = "/sys/class/fpga"
	devfsDirectory = "/dev"
	deviceRE       = `^intel-fpga-dev.[0-9]+$`
	portRE         = `^intel-fpga-port.[0-9]+$`
	fmeRE          = `^intel-fpga-fme.[0-9]+$`
)

// UpdateInfo contains info added, updated and deleted FPGA devices
type UpdateInfo struct {
	Added   map[string]map[string]deviceplugin.DeviceInfo
	Updated map[string]map[string]deviceplugin.DeviceInfo
	Removed map[string]map[string]deviceplugin.DeviceInfo
}

type getDevMapFunc func(devices []device) map[string]map[string]deviceplugin.DeviceInfo

func getRegionMap(devices []device) map[string]map[string]deviceplugin.DeviceInfo {
	regionMap := make(map[string]map[string]deviceplugin.DeviceInfo)

	for _, dev := range devices {
		for _, region := range dev.regions {
			if _, present := regionMap[region.interfaceID]; !present {
				regionMap[region.interfaceID] = make(map[string]deviceplugin.DeviceInfo)
			}
			devNodes := make([]string, len(region.afus)+1)
			for num, afu := range region.afus {
				devNodes[num] = afu.devNode
			}
			devNodes[len(region.afus)] = region.devNode
			regionMap[region.interfaceID][region.id] = deviceplugin.DeviceInfo{
				State: pluginapi.Healthy,
				Nodes: devNodes,
			}
		}
	}

	return regionMap
}

func getAfuMap(devices []device) map[string]map[string]deviceplugin.DeviceInfo {
	afuMap := make(map[string]map[string]deviceplugin.DeviceInfo)

	for _, dev := range devices {
		for _, region := range dev.regions {
			for _, afu := range region.afus {
				if _, present := afuMap[afu.afuID]; !present {
					afuMap[afu.afuID] = make(map[string]deviceplugin.DeviceInfo)
				}
				afuMap[afu.afuID][afu.id] = deviceplugin.DeviceInfo{
					State: pluginapi.Healthy,
					Nodes: []string{afu.devNode},
				}
			}
		}
	}

	return afuMap
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

// Cache represents FPGA devices found on the host
type Cache struct {
	sysfsDir string
	devfsDir string

	deviceReg *regexp.Regexp
	portReg   *regexp.Regexp
	fmeReg    *regexp.Regexp

	devices   []device
	ch        chan<- UpdateInfo
	getDevMap getDevMapFunc
}

// NewCache returns new instance of Cache
func NewCache(sysfsDir string, devfsDir string, mode string, ch chan<- UpdateInfo) (*Cache, error) {
	var getDevMap getDevMapFunc

	switch mode {
	case AfMode:
		getDevMap = getAfuMap
	case RegionMode:
		getDevMap = getRegionMap
	default:
		return nil, fmt.Errorf("Wrong mode: '%s'", mode)
	}

	return &Cache{
		sysfsDir:  sysfsDir,
		devfsDir:  devfsDir,
		deviceReg: regexp.MustCompile(deviceRE),
		portReg:   regexp.MustCompile(portRE),
		fmeReg:    regexp.MustCompile(fmeRE),
		ch:        ch,
		getDevMap: getDevMap,
	}, nil
}

func (c *Cache) getDevNode(devName string) (string, error) {
	devNode := path.Join(c.devfsDir, devName)
	if _, err := os.Stat(devNode); err != nil {
		return "", fmt.Errorf("Device %s doesn't exist: %v", devNode, err)
	}

	return devNode, nil
}

func (c *Cache) detectUpdates(devices []device) {
	added := make(map[string]map[string]deviceplugin.DeviceInfo)
	updated := make(map[string]map[string]deviceplugin.DeviceInfo)

	oldDevMap := c.getDevMap(c.devices)

	for fpgaID, new := range c.getDevMap(devices) {
		if old, ok := oldDevMap[fpgaID]; ok {
			if !reflect.DeepEqual(old, new) {
				updated[fpgaID] = new
			}
			delete(oldDevMap, fpgaID)
		} else {
			added[fpgaID] = new
		}
	}

	if len(added) > 0 || len(updated) > 0 || len(oldDevMap) > 0 {
		c.ch <- UpdateInfo{
			Added:   added,
			Updated: updated,
			Removed: oldDevMap,
		}
	}
}

func (c *Cache) scanFPGAs() error {
	var devices []device

	glog.V(2).Info("Start new FPGA scan")

	fpgaFiles, err := ioutil.ReadDir(c.sysfsDir)
	if err != nil {
		return fmt.Errorf("Can't read sysfs folder: %v", err)
	}

	for _, fpgaFile := range fpgaFiles {
		fname := fpgaFile.Name()

		if !c.deviceReg.MatchString(fname) {
			continue
		}

		deviceFolder := path.Join(c.sysfsDir, fname)
		deviceFiles, err := ioutil.ReadDir(deviceFolder)
		if err != nil {
			return err
		}

		var regions []region
		var afus []afu
		for _, deviceFile := range deviceFiles {
			name := deviceFile.Name()

			if c.fmeReg.MatchString(name) {
				if len(regions) > 0 {
					return fmt.Errorf("Detected more than one FPGA region for device %s. Only one region per FPGA device is supported", fname)
				}
				interfaceIDFile := path.Join(deviceFolder, name, "pr", "interface_id")
				data, err := ioutil.ReadFile(interfaceIDFile)
				if err != nil {
					return err
				}
				devNode, err := c.getDevNode(name)
				if err != nil {
					return err
				}
				regions = append(regions, region{
					id:          name,
					interfaceID: strings.TrimSpace(string(data)),
					devNode:     devNode,
				})
			} else if c.portReg.MatchString(name) {
				afuFile := path.Join(deviceFolder, name, "afu_id")
				data, err := ioutil.ReadFile(afuFile)
				if err != nil {
					return err
				}
				devNode, err := c.getDevNode(name)
				if err != nil {
					return err
				}
				afus = append(afus, afu{
					id:      name,
					afuID:   strings.TrimSpace(string(data)),
					devNode: devNode,
				})
			}
		}

		if len(regions) == 0 {
			return fmt.Errorf("No regions found for device %s", fname)
		}

		// Currently only one region per device is supported.
		regions[0].afus = afus
		devices = append(devices, device{
			name:    fname,
			regions: regions,
		})
	}

	c.detectUpdates(devices)
	c.devices = devices

	return nil
}

// Run starts scanning of FPGA devices on the host
func (c *Cache) Run() error {
	for {
		err := c.scanFPGAs()
		if err != nil {
			glog.Error("Device scan failed: ", err)
			return fmt.Errorf("Device scan failed: %v", err)
		}

		time.Sleep(5 * time.Second)
	}
}
