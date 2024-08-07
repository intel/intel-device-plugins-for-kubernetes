#!/bin/sh -eu
#
# Copyright 2019-2021 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Invoke this script with a version as parameter
# and it will update all hard-coded image versions
# in the source code.
#
# Adapted from https://github.com/intel/pmem-csi/

if [ $# != 1 ] || [ "$1" = "?" ] || [ "$1" = "--help" ]; then
    echo "Usage: $0 <image version>" >&2
    exit 1
fi

files=$(git grep -l '^TAG?*=\|intel/accel-config-demo:\|intel/crypto-perf:\|intel/opae-nlb-demo:\|intel/openssl-qat-engine:\|intel/dlb-libdlb-demo:\|intel/stress-ng-gramine:\|intel/sgx-sdk-demo:\|intel/intel-[^ ]*:\|version=\|appVersion:\|tag:' Makefile deployments demo/*accel-config*.yaml demo/*fpga*.yaml demo/*openssl*.yaml demo/dlb-libdlb*.yaml pkg/controllers/*/*_test.go build/docker/*.Dockerfile test/e2e/*/*.go)

for file in $files; do
    sed -i -e "s;\(^TAG?*=\|intel/accel-config-demo:\|intel/crypto-perf:\|intel/opae-nlb-demo:\|intel/openssl-qat-engine:\|intel/dlb-libdlb-demo:\|intel/stress-ng-gramine:\|intel/sgx-sdk-demo:\|intel/intel-[^ ]*:\|version=\|appVersion: [^ ]\|tag: [^ ]\)[^ \"]*;\1$1;g" "$file";
done
