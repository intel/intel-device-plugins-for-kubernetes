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
	"os"

	"github.com/golang/glog"
	"golang.org/x/net/context"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

	"github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/devicecache"
	"github.com/intel/intel-device-plugins-for-kubernetes/internal/deviceplugin"
)

const (
	sysfsDirectory = "/sys/class/fpga"
	devfsDirectory = "/dev"

	// Device plugin settings.
	pluginEndpointPrefix = "intel-fpga"
	resourceNamePrefix   = "intel.com/fpga"
)

// deviceManager manages Intel FPGA devices.
type deviceManager struct {
	srv     deviceplugin.Server
	fpgaID  string
	name    string
	ch      chan map[string]deviceplugin.DeviceInfo
	devices map[string]deviceplugin.DeviceInfo
}

func newDeviceManager(resourceName string, fpgaID string, ch chan map[string]deviceplugin.DeviceInfo) *deviceManager {
	return &deviceManager{
		fpgaID:  fpgaID,
		name:    resourceName,
		ch:      ch,
		devices: make(map[string]deviceplugin.DeviceInfo),
	}
}

func (dm *deviceManager) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	fmt.Println("GetDevicePluginOptions: return empty options")
	return new(pluginapi.DevicePluginOptions), nil
}

// ListAndWatch returns a list of devices
// Whenever a device state change or a device disappears, ListAndWatch returns the new list
func (dm *deviceManager) ListAndWatch(empty *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	glog.V(2).Info("Started ListAndWatch for ", dm.fpgaID)

	for dm.devices = range dm.ch {
		resp := new(pluginapi.ListAndWatchResponse)
		for id, device := range dm.devices {
			resp.Devices = append(resp.Devices, &pluginapi.Device{id, device.State})
		}
		glog.V(2).Info("Sending to kubelet ", resp.Devices)
		if err := stream.Send(resp); err != nil {
			dm.srv.Stop()
			return fmt.Errorf("device-plugin: cannot update device list: %v", err)
		}
	}

	return nil
}

func (dm *deviceManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	return deviceplugin.MakeAllocateResponse(rqt, dm.devices)
}

func (dm *deviceManager) PreStartContainer(ctx context.Context, rqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	glog.Warning("PreStartContainer() should not be called")
	return new(pluginapi.PreStartContainerResponse), nil
}

func startDeviceManager(dm *deviceManager, pluginPrefix string) {
	err := dm.srv.Serve(dm, dm.name, pluginPrefix)
	if err != nil {
		glog.Fatal(err)
	}
}

func handleUpdate(dms map[string]*deviceManager, updateInfo devicecache.UpdateInfo, start func(*deviceManager, string)) {
	glog.V(2).Info("Recieved dev updates: ", updateInfo)
	for fpgaID, devices := range updateInfo.Added {
		devCh := make(chan map[string]deviceplugin.DeviceInfo, 1)
		resourceName := resourceNamePrefix + "-" + fpgaID
		pPrefix := pluginEndpointPrefix + "-" + fpgaID
		dms[fpgaID] = newDeviceManager(resourceName, fpgaID, devCh)
		go start(dms[fpgaID], pPrefix)
		devCh <- devices
	}
	for fpgaID, devices := range updateInfo.Updated {
		dms[fpgaID].ch <- devices
	}
	for fpgaID := range updateInfo.Removed {
		dms[fpgaID].srv.Stop()
		close(dms[fpgaID].ch)
		delete(dms, fpgaID)
	}
}

func main() {
	var mode string

	flag.StringVar(&mode, "mode", string(devicecache.AfMode),
		fmt.Sprintf("device plugin mode: '%s' (default), '%s' or '%s'", devicecache.AfMode, devicecache.RegionMode, devicecache.RegionDevelMode))
	flag.Parse()

	updatesCh := make(chan devicecache.UpdateInfo)

	cache, err := devicecache.NewCache(sysfsDirectory, devfsDirectory, mode, updatesCh)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}

	glog.Info("FPGA device plugin started in ", mode, " mode")

	go func() {
		err := cache.Run()
		if err != nil {
			glog.Fatal(err)
		}
	}()

	dms := make(map[string]*deviceManager)
	for updateInfo := range updatesCh {
		handleUpdate(dms, updateInfo, startDeviceManager)
	}
}
