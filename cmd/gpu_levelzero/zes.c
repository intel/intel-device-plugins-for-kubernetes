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
#include <sys/time.h>
#include <stdlib.h>

#include <zes_api.h>

#include "ze.h"

#define MAX_BDF_BUFSIZE 32

struct device_info {
    char bdf[MAX_BDF_BUFSIZE];
};

zes_device_handle_t* zes_handles = NULL;
struct device_info* bdf_addresses = NULL;
uint32_t zes_handles_count = 0;

static bool device_enumerated = false;

bool zes_try_initialize(void)
{
    return zesInit(0) == ZE_RESULT_SUCCESS;
}

static ze_result_t enumerate_zes_devices(void)
{
    ze_result_t res = zesInit(0);
    if (res != ZE_RESULT_SUCCESS) {
        return res;
    }

    uint32_t count = 0;

    res = zesDriverGet(&count, NULL);
    if (res != ZE_RESULT_SUCCESS) {
        return res;
    }

    if (count == 0) {
        return ZE_RESULT_ERROR_NOT_AVAILABLE;
    }

    if (count > 1) {
        fprintf(stderr, "more than one zes driver detected, using first one\n");
        fflush(stderr);
    }

    count = 1;

    zes_driver_handle_t handle;
    res = zesDriverGet(&count, &handle);
    if (res != ZE_RESULT_SUCCESS) {
        return res;
    }

    count = 0;
    res = zesDeviceGet(handle, &count, NULL);
    if (res != ZE_RESULT_SUCCESS) {
        return res;
    }

    if (count == 0) {
        return ZE_RESULT_ERROR_NOT_AVAILABLE;
    }

    zes_handles = calloc(count, sizeof(zes_device_handle_t));

    res = zesDeviceGet(handle, &count, zes_handles);
    if (res != ZE_RESULT_SUCCESS) {
        free(zes_handles);

        return res;
    }

    zes_handles_count = count;

    bdf_addresses = (struct device_info*) calloc(count,sizeof(struct device_info));
    if (bdf_addresses == NULL) {
        free(zes_handles);

        return ZE_RESULT_ERROR_OUT_OF_HOST_MEMORY;
    }

    // Iterate over the devices and store their info into the cache array
    for (uint32_t i = 0; i < count; ++i) {
        zes_device_handle_t dev_h = zes_handles[i];

        zes_pci_properties_t pci_props;
        if (zesDevicePciGetProperties(dev_h, &pci_props) != ZE_RESULT_SUCCESS) {
            continue;
        }

        zes_pci_address_t* addr = &pci_props.address;

        snprintf(bdf_addresses[i].bdf, sizeof(bdf_addresses[i].bdf),
            "%04x:%02x:%02x.%x",
            addr->domain, addr->bus, addr->device, addr->function
        );
    }

    device_enumerated = true;

    return res;
}

zes_device_handle_t retrieve_handle_for_bdf(char* bdf_address)
{
    zes_device_handle_t handle = 0;

    for (uint32_t i = 0; i < zes_handles_count; ++i) {
        struct device_info* di = &bdf_addresses[i];

        if (strncmp(bdf_address, di->bdf, sizeof(di->bdf)) == 0) {
            handle = zes_handles[i];
            break;
        }
    }

    return handle;
}

bool is_integrated(zes_device_handle_t handle)
{
    ze_result_t res = ZE_RESULT_SUCCESS;

    zes_device_ext_properties_t ext = {
        .stype = ZES_STRUCTURE_TYPE_DEVICE_EXT_PROPERTIES,
    };
    zes_device_properties_t props = {
        .stype = ZES_STRUCTURE_TYPE_DEVICE_PROPERTIES,
        .pNext = &ext,
    };

    if (res = zesDeviceGetProperties(handle, &props), res == ZE_RESULT_SUCCESS) {
        if (ext.flags & ZES_DEVICE_PROPERTY_FLAG_INTEGRATED) {
            return true;
        }
    }

    return false;
}

/// @brief Retrieves memory amount for a specific device with bdf address
/// @param bdf_address
/// @return memory amount for the device
uint64_t zes_device_memory_amount(char* bdf_address, uint32_t* error)
{
    ze_result_t res = ZE_RESULT_SUCCESS;

    if (!device_enumerated) {
        res = enumerate_zes_devices();
        if (res != ZE_RESULT_SUCCESS) {
            *error = res;

            return 0;
        }
    }

    zes_device_handle_t handle = retrieve_handle_for_bdf(bdf_address);
    if (handle == 0) {
        *error = ZE_RESULT_ERROR_UNKNOWN;

        return 0;
    }

    // Levelzero does not provide memory details for integrated
    if (is_integrated(handle)) {
        return 0;
    }

    uint32_t modcount = 0;
    uint64_t memory_size = 0;
    if (!zesDeviceEnumMemoryModules(handle, &modcount, NULL) == ZE_RESULT_SUCCESS && modcount > 0) {
        zes_mem_handle_t memhandles[modcount];

        if (zesDeviceEnumMemoryModules(handle, &modcount, memhandles) == ZE_RESULT_SUCCESS) {
            for (uint32_t mod_index = 0; mod_index < modcount; ++mod_index) {
                zes_mem_state_t mem_state;

                if (zesMemoryGetState(memhandles[mod_index], &mem_state) == ZE_RESULT_SUCCESS) {
                    memory_size += mem_state.size;
                }
            }
        }
    }

    return memory_size;
}

/// @brief Retrieve device memory's health status
/// @param bdf_address
/// @return true for good, false for bad
bool zes_device_memory_is_healthy(char* bdf_address, uint32_t* error)
{
    if (!device_enumerated) {
        ze_result_t res = enumerate_zes_devices();
        if (res != ZE_RESULT_SUCCESS) {
            *error = res;

            return true;
        }
    }

    zes_device_handle_t handle = retrieve_handle_for_bdf(bdf_address);
    if (handle == 0) {
        *error = ZE_RESULT_ERROR_UNKNOWN;

        return true;
    }

    // Levelzero does not provide memory details for integrated
    if (is_integrated(handle)) {
        return true;
    }

    uint32_t modcount = 0;
    if (zesDeviceEnumMemoryModules(handle, &modcount, NULL) == ZE_RESULT_SUCCESS && modcount > 0) {
        zes_mem_handle_t memhandles[modcount];

        if (zesDeviceEnumMemoryModules(handle, &modcount, memhandles) == ZE_RESULT_SUCCESS) {
            for (uint32_t mod_index = 0; mod_index < modcount; ++mod_index) {
                zes_mem_state_t mem_state;

                if (zesMemoryGetState(memhandles[mod_index], &mem_state) == ZE_RESULT_SUCCESS) {
                    if (mem_state.health >= ZES_MEM_HEALTH_CRITICAL) {
                        return false;
                    }
                }
            }
        }
    }

    return true;
}

/// @brief Retrieve device bus' health status
/// @param bdf_address
/// @return true for good, false for bad
bool zes_device_bus_is_healthy(char* bdf_address, uint32_t* error)
{
    if (!device_enumerated) {
        ze_result_t res = enumerate_zes_devices();
        if (res != ZE_RESULT_SUCCESS) {
            *error = res;

            return res;
        }
    }

    zes_device_handle_t handle = retrieve_handle_for_bdf(bdf_address);
    if (handle == 0) {
        *error = ZE_RESULT_ERROR_UNKNOWN;

        return 0;
    }

    zes_pci_state_t pci_state;
    memset(&pci_state, 0, sizeof(pci_state));

    ze_result_t res = zesDevicePciGetState(handle, &pci_state);
    if (res == ZE_RESULT_SUCCESS) {
        if (pci_state.qualityIssues & ZES_PCI_LINK_QUAL_ISSUE_FLAG_SPEED) {
            return false;
        }
    } else if (res != ZE_RESULT_ERROR_UNSUPPORTED_FEATURE) {
        *error = res;
    }

    return true;
}
