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

#include <stdio.h>
#include <string.h>
#include <stdlib.h>

#include <ze_api.h>

#include "ze.h"

int ze_status_to_string(const uint32_t error, char* out, uint32_t out_size)
{
    char* description;

    switch (error) {
        case ZE_RESULT_SUCCESS:
            description = "success"; break;
        case ZE_RESULT_NOT_READY:
            description = "not ready"; break;
        case ZE_RESULT_ERROR_DEVICE_LOST:
            description = "device lost"; break;
        case ZE_RESULT_ERROR_DEVICE_REQUIRES_RESET:
            description = "device requires reset"; break;
        case ZE_RESULT_ERROR_DEVICE_IN_LOW_POWER_STATE:
            description = "device in low power state"; break;
        case ZE_RESULT_ERROR_INSUFFICIENT_PERMISSIONS:
            description = "insufficient permissions"; break;
        case ZE_RESULT_ERROR_NOT_AVAILABLE:
            description = "not available"; break;
        case ZE_RESULT_ERROR_DEPENDENCY_UNAVAILABLE:
            description = "dependency unavailable"; break;
        case ZE_RESULT_ERROR_UNINITIALIZED:
            description = "uninitialized"; break;
        case ZE_RESULT_ERROR_UNSUPPORTED_VERSION:
            description = "unsupported version"; break;
        case ZE_RESULT_ERROR_UNSUPPORTED_FEATURE:
            description = "unsupported feature"; break;
        case ZE_RESULT_ERROR_INVALID_ARGUMENT:
            description = "invalid argument"; break;
        case ZE_RESULT_ERROR_INVALID_NULL_POINTER:
            description = "invalid null pointer"; break;
        case ZE_RESULT_ERROR_INVALID_NULL_HANDLE:
            description = "invalid null handle"; break;
        case ZE_RESULT_ERROR_UNKNOWN:
            description = "unknown"; break;
        default:
            description = "not known"; break;
    }

    return snprintf(out, out_size -1, "%s (0x%X)", description, error);
}

static ze_driver_handle_t initialize_ze(void)
{
    ze_result_t res = zeInit(ZE_INIT_FLAG_GPU_ONLY);
    if (res != ZE_RESULT_SUCCESS) {
        fprintf(stderr, "zeInit failed: 0x%X \n", res);

        return 0;
    }

    uint32_t count = 0;

    if (zeDriverGet(&count, NULL) != ZE_RESULT_SUCCESS || count == 0) {
        fprintf(stderr, "zeDriverGet failed or no drivers\n");

        return 0;
    }

    count = 1;

    ze_driver_handle_t handle;
    if (zeDriverGet(&count, &handle) != ZE_RESULT_SUCCESS) {
        fprintf(stderr, "zeDriverGet failed\n");

        return 0;
    }

    return handle;
}

bool ze_try_initialize(void)
{
    if (getenv("UNITTEST") != NULL) {
        return false;
    }

    return zeInit(0) == ZE_RESULT_SUCCESS;
}

/// @brief Retrieve indices for Intel levelzero devices
/// @param indices Pointer to an array to store indices
/// @param indices_size Size of the array
/// @return Number of indices stored
int ze_intel_device_indices(uint32_t* indices, uint32_t indices_size, uint32_t *error)
{
    if (getenv("UNITTEST") != NULL) {
        return 0;
    }

    if (indices == NULL || 0 == indices_size) {
        *error = ZE_RESULT_ERROR_INVALID_NULL_POINTER;

        return 0;
    }

    ze_driver_handle_t handle = initialize_ze();

    if (handle == 0) {
        *error = ZE_RESULT_ERROR_INVALID_NULL_POINTER;

        return 0;
    }

    ze_result_t res = 0;
    uint32_t count = 0;

    res = zeDeviceGet(handle, &count, NULL);
    if (res != ZE_RESULT_SUCCESS) {
        *error = res;

        return 0;
    }

    if (count == 0) {
        *error = ZE_RESULT_ERROR_DEVICE_LOST;

        return 0;
    }

    ze_device_handle_t dev_handle[count];

    res = zeDeviceGet(handle, &count, dev_handle);
    if (res != ZE_RESULT_SUCCESS) {
        *error = res;

        return 0;
    }

    if (count > indices_size) {
        count = indices_size;
    }

    int intel_device_count = 0;

    // Iterate over the devices and add Intel indices to be returned
    for (uint32_t i = 0; i < count; ++i) {
        ze_device_handle_t dev_h = dev_handle[i];

        ze_device_properties_t dev_prop;
        memset(&dev_prop, 0, sizeof(ze_device_properties_t));

        res = zeDeviceGetProperties(dev_h, &dev_prop);
        if (res != ZE_RESULT_SUCCESS) {
            continue;
        }

        if (dev_prop.vendorId == VENDOR_ID_INTEL) {
            indices[intel_device_count] = i;
            intel_device_count++;
        }
    }

    return intel_device_count;
}
