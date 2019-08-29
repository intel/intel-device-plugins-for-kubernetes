#!/bin/sh
#
# Copyright 2019 Intel Corporation.
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

sed -i -e "s;\(^TAG?*=\|intel/crypto-perf:\|intel/intel-[^ ]*:\)[^ ]*;\1$1;g" $(git grep -l '^TAG?*=\|intel/crypto-perf:\|intel/intel-[^ ]*:' Makefile deployments)
