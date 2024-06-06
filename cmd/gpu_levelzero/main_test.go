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

import (
	"context"
	"testing"

	levelzero "github.com/intel/intel-device-plugins-for-kubernetes/cmd/internal/levelzero"
)

func TestErrorConversion(t *testing.T) {
	t.Run("Known conversion(s)", func(t *testing.T) {
		desc := retrieveStatusDescription(0)

		if desc != "success (0x0)" {
			t.Fatal("couldn't convert 0 to success: ", desc)
		}

		desc = retrieveStatusDescription(1879048193) // device lost

		if desc != "device lost (0x70000001)" {
			t.Fatal("couldn't convert 0 to success: ", desc)
		}
	})
}

func TestCallingMethods(t *testing.T) {
	s := server{}

	// As we cannot control the testing environment, we can't really check the return values for any sane values.

	t.Run("Call get indices", func(t *testing.T) {
		indices, err := s.GetIntelIndices(context.Background(), &levelzero.GetIntelIndicesMessage{})

		if len(indices.Indices) == 0 {
			t.Log("No indices received")
		}
		if err != nil {
			t.Log("Received an error")
		}
	})

	t.Run("Call get health", func(t *testing.T) {
		health, err := s.GetDeviceHealth(context.Background(), &levelzero.DeviceId{BdfAddress: "0000:00:01.0"})

		if health.MemoryOk {
			t.Log("Memory is ok")
		}
		if err != nil {
			t.Log("Received an error")
		}
	})

	t.Run("Call get temperature", func(t *testing.T) {
		temps, err := s.GetDeviceTemperature(context.Background(), &levelzero.DeviceId{BdfAddress: "0000:00:01.0"})

		if temps.Global > -999.0 {
			t.Log("Memory is ok")
		}
		if err != nil {
			t.Log("Received an error")
		}
	})

	t.Run("Call get memory", func(t *testing.T) {
		amount, err := s.GetDeviceMemoryAmount(context.Background(), &levelzero.DeviceId{BdfAddress: "0000:00:01.0"})

		if amount.MemorySize > 0 {
			t.Log("Received some memory")
		}
		if err != nil {
			t.Log("Received an error")
		}
	})
}
