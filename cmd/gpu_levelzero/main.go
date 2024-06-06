// Copyright 2024 Intel Corporation. All Rights Reserved.
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

// #cgo CFLAGS: "-I/usr/include/level_zero" "-Wall" "-Wextra" "-O2"
// #cgo LDFLAGS: "-lze_loader"
// #include "ze.h"
import "C"

import (
	"context"
	"flag"
	"net"
	"os"
	"strconv"
	"unsafe"

	levelzero "github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/levelzero"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
)

type server struct {
	levelzero.UnimplementedLevelzeroServer
}

func retrieveStatusDescription(code uint32) string {
	bSize := 64
	b := make([]byte, bSize)

	cwritten := C.ze_status_to_string(C.uint32_t(code), (*C.char)(unsafe.Pointer(&b[0])), C.uint32_t(bSize))

	written := int(cwritten)
	if written <= 0 {
		return "failed to retrieve description"
	}

	return string(b[0:written])
}

func (s *server) GetDeviceHealth(c context.Context, deviceid *levelzero.DeviceId) (*levelzero.DeviceHealth, error) {
	klog.V(3).Infof("Retrieve device health for %s", deviceid.BdfAddress)

	var errorVal uint32 = 0

	cBdfAddress := C.CString(deviceid.BdfAddress)

	memHealth := bool(C.zes_device_memory_is_healthy(cBdfAddress, (*C.uint32_t)(unsafe.Pointer(&errorVal))))
	if errorVal != 0 {
		klog.Warningf("device memory health read returned an error: 0x%X", errorVal)
	}

	busHealth := bool(C.zes_device_bus_is_healthy(cBdfAddress, (*C.uint32_t)(unsafe.Pointer(&errorVal))))
	if errorVal != 0 {
		klog.Warningf("device bus health read returned an error: 0x%X", errorVal)
	}

	var err levelzero.Error
	if errorVal != 0 {
		err.Errorcode = errorVal
		err.Description = retrieveStatusDescription(errorVal)
	} else {
		klog.V(3).Infof("Health for %s: Memory=%t, Bus=%t", deviceid.BdfAddress, memHealth, busHealth)
	}

	health := &levelzero.DeviceHealth{
		BusOk:    busHealth,
		MemoryOk: memHealth,
		SocOk:    true, // Placeholder, not available.
		Error:    &err,
	}

	return health, nil
}

func (s *server) GetDeviceTemperature(c context.Context, deviceid *levelzero.DeviceId) (*levelzero.DeviceTemperature, error) {
	klog.V(3).Infof("Retrieve device temperature for %s", deviceid.BdfAddress)

	var errorVal uint32 = 0

	cBdfAddress := C.CString(deviceid.BdfAddress)

	globalTemp := float64(C.zes_device_temp_max(cBdfAddress, C.CString("global"), (*C.uint32_t)(unsafe.Pointer(&errorVal))))
	if errorVal != 0 {
		klog.Warningf("global temperature read returned an error: 0x%X", errorVal)
	}

	gpuTemp := float64(C.zes_device_temp_max(cBdfAddress, C.CString("gpu"), (*C.uint32_t)(unsafe.Pointer(&errorVal))))
	if errorVal != 0 {
		klog.Warningf("gpu temperature read returned an error: 0x%X", errorVal)
	}

	memTemp := float64(C.zes_device_temp_max(cBdfAddress, C.CString("memory"), (*C.uint32_t)(unsafe.Pointer(&errorVal))))
	if errorVal != 0 {
		klog.Warningf("memory temperature read returned an error: 0x%X", errorVal)
	}

	var err levelzero.Error
	if errorVal != 0 {
		err.Errorcode = errorVal
		err.Description = retrieveStatusDescription(errorVal)
	} else {
		klog.V(3).Infof("Temperatures for %s: Memory=%.1fC, GPU=%.1fC, Global=%.1fC", deviceid.BdfAddress, memTemp, gpuTemp, globalTemp)
	}

	temps := &levelzero.DeviceTemperature{
		Global: globalTemp,
		Gpu:    gpuTemp,
		Memory: memTemp,
		Error:  &err,
	}

	return temps, nil
}

func (s *server) GetIntelIndices(c context.Context, m *levelzero.GetIntelIndicesMessage) (*levelzero.DeviceIndices, error) {
	klog.V(3).Infof("Retrieve Intel indices")

	errorVal := uint32(0)

	indices := make([]uint32, 8)

	// TODO: Move to zes_ version when crash in WSL env is fixed:
	// https://github.com/intel/compute-runtime/issues/721
	count := C.ze_intel_device_indices((*C.uint32_t)(&indices[0]), C.uint32_t(len(indices)), (*C.uint32_t)(unsafe.Pointer(&errorVal)))

	var err levelzero.Error
	if errorVal != 0 {
		err.Errorcode = errorVal
		err.Description = retrieveStatusDescription(errorVal)
	}

	ret := levelzero.DeviceIndices{
		Indices: indices[0:count],
		Error:   &err,
	}

	return &ret, nil
}

func (s *server) GetDeviceMemoryAmount(c context.Context, deviceid *levelzero.DeviceId) (*levelzero.DeviceMemoryAmount, error) {
	klog.V(3).Infof("Retrieve device memory amount for %s", deviceid.BdfAddress)

	errorVal := uint32(0)

	memSize := C.zes_device_memory_amount(C.CString(deviceid.BdfAddress), (*C.uint32_t)(unsafe.Pointer(&errorVal)))

	if errorVal != 0 {
		klog.Warningf("device memory amount read returned an error: 0x%X", errorVal)
	}

	description := retrieveStatusDescription(errorVal)

	var err levelzero.Error
	if errorVal != 0 {
		err.Errorcode = errorVal
		err.Description = description
	}

	ret := levelzero.DeviceMemoryAmount{
		MemorySize: uint64(memSize),
		Error:      &err,
	}

	return &ret, nil
}

func main() {
	klog.InitFlags(nil)

	socketPath := flag.String("socket", levelzero.DefaultUnixSocketPath, "Unix socket path to listen on")
	wslEnv := flag.Bool("wsl", false, "Running in WSL environment")

	flag.Parse()

	// Delete possible previous socket file
	_ = os.Remove(*socketPath)

	verbosity := int64(0)

	flag.VisitAll(func(f *flag.Flag) {
		if f.Name == "v" {
			if v, err := strconv.ParseInt(f.Value.String(), 10, 16); err == nil {
				verbosity = v
			}
		}
	})

	lis, err := net.Listen("unix", *socketPath)
	if err != nil {
		klog.Fatalf("failed to listen: %v", err)
	}

	// TODO: Drop "ze_try_initialize" when crash in WSL env is fixed:
	// https://github.com/intel/compute-runtime/issues/721
	if *wslEnv {
		if !bool(C.ze_try_initialize()) {
			klog.Fatal("Ze Init try failed, cannot continue")
		}
	} else {
		if !bool(C.zes_try_initialize()) {
			klog.Fatal("Zes Init try failed, cannot continue")
		}
	}

	C.zes_set_verbosity(C.int(verbosity))

	s := grpc.NewServer()

	levelzero.RegisterLevelzeroServer(s, &server{})

	klog.Infof("server listening at %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		klog.Fatalf("failed to serve: %v", err)
	}
}
