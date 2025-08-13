#!/bin/bash

echo "Run NPU tests"

npu-kmd-test *:-Device.GroupOwnership || exit 1

# use npu-umd-test -l to list all topics
# note: some of the tests require the NPU compiler & openvino runtime
# https://github.com/intel/linux-npu-driver/blob/main/docs/overview.md#building-a-driver-together-with-the-compiler
topics="Device Driver Event EventSync MemoryAllocation MemoryExecution MemoryAllocationThreaded CommandCopyPerf MultiMemoryExecution MultiContext CommandMemoryFill"

for topic in $topics; do
    npu-umd-test -v ${topic}.* || exit 1
done

echo "Tests done"

exit 0
