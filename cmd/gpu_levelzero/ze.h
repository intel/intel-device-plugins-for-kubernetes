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

#pragma once

#include <stdint.h>
#include <stdbool.h>

#define VENDOR_ID_INTEL 0x8086
#define TEMP_ERROR_RET_VAL -999.0

void zes_set_verbosity(const int level);

bool ze_try_initialize(void);
bool zes_try_initialize(void);

int ze_status_to_string(const uint32_t error, char* out, uint32_t out_size);

int ze_intel_device_indices(uint32_t* indices, uint32_t indices_size, uint32_t* error);
uint64_t zes_device_memory_amount(char* bdf_address, uint32_t* error);
bool zes_device_memory_is_healthy(char* bdf_address, uint32_t* error);
bool zes_device_bus_is_healthy(char* bdf_address, uint32_t* error);
double zes_device_temp_max(char* bdf_address, char* sensor, uint32_t* error);
