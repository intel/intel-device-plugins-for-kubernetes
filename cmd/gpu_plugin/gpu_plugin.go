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
	"golang.org/x/net/context"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

const (
	sysfsDrmDirectory = "/sys/class/drm"
	devfsDriDirectory = "/dev/dri"
	gpuDeviceRE       = `^card[0-9]+$`
	vendorString      = "0x8086"

	// Device plugin settings.
	pluginEndpointPrefix = "intelGPU"
	resourceName         = "intel.com/gpu"
)

// deviceManager manages Intel gpu devices.
type deviceManager struct {
	srv     deviceplugin.Server
	devices map[string]deviceplugin.DeviceInfo
}

func newDeviceManager() *deviceManager {
	return &deviceManager{
		devices: make(map[string]deviceplugin.DeviceInfo),
	}
}

// Discovers all GPU devices available on the local node by walking `/sys/class/drm` directory.
func (dm *deviceManager) discoverGPUs(sysfsDrmDir string, devfsDriDir string) error {
	reg := regexp.MustCompile(gpuDeviceRE)
	files, err := ioutil.ReadDir(sysfsDrmDir)
	if err != nil {
		return fmt.Errorf("Can't read sysfs folder: %v", err)
	}
	for _, f := range files {
		if reg.MatchString(f.Name()) {
			dat, err := ioutil.ReadFile(path.Join(sysfsDrmDir, f.Name(), "device/vendor"))
			if err != nil {
				fmt.Println("Oops can't read vendor file")
				continue
			}

			if strings.TrimSpace(string(dat)) == vendorString {
				var nodes []string

				drmFiles, err := ioutil.ReadDir(path.Join(sysfsDrmDir, f.Name(), "device/drm"))
				if err != nil {
					return fmt.Errorf("Can't read device folder: %v", err)
				}

				for _, drmFile := range drmFiles {
					devPath := path.Join(devfsDriDir, drmFile.Name())
					if _, err := os.Stat(devPath); err != nil {
						continue
					}

					fmt.Printf("Adding '%s' to GPU '%s'\n", devPath, f.Name())
					nodes = append(nodes, devPath)
				}

				if len(nodes) > 0 {
					dm.devices[f.Name()] = deviceplugin.DeviceInfo{pluginapi.Healthy, nodes}
				}
			}
		}
	}

	return nil
}

func (dm *deviceManager) getDeviceState(DeviceName string) string {
	// TODO: calling tools to figure out actual device state
	return pluginapi.Healthy
}

// Implements DevicePlugin service functions
func (dm *deviceManager) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	fmt.Println("GetDevicePluginOptions: return empty options")
	return new(pluginapi.DevicePluginOptions), nil
}

func (dm *deviceManager) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	fmt.Println("device-plugin: ListAndWatch start")
	changed := true
	for {
		for id, dev := range dm.devices {
			state := dm.getDeviceState(id)
			if dev.State != state {
				changed = true
				dev.State = state
				dm.devices[id] = dev
			}
		}
		if changed {
			resp := new(pluginapi.ListAndWatchResponse)
			for id, dev := range dm.devices {
				resp.Devices = append(resp.Devices, &pluginapi.Device{id, dev.State})
			}
			fmt.Printf("ListAndWatch: send devices %v\n", resp)
			if err := stream.Send(resp); err != nil {
				dm.srv.Stop()
				return fmt.Errorf("device-plugin: cannot update device states: %v", err)
			}
		}
		changed = false
		time.Sleep(5 * time.Second)
	}
}

func (dm *deviceManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return deviceplugin.MakeAllocateResponse(rqt, dm.devices)
}

func (dm *deviceManager) PreStartContainer(ctx context.Context, rqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	glog.Warning("PreStartContainer() should not be called")
	return new(pluginapi.PreStartContainerResponse), nil
}

func main() {
	flag.Parse()
	fmt.Println("GPU device plugin started")
	dm := newDeviceManager()

	err := dm.discoverGPUs(sysfsDrmDirectory, devfsDriDirectory)
	if err != nil {
		glog.Fatal(err)
	}

	err = dm.srv.Serve(dm, resourceName, pluginEndpointPrefix)
	if err != nil {
		glog.Fatal(err)
	}
}
