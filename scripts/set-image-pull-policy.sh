#!/usr/bin/env bash
#
# Copyright 2019-2020 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Invoke this script with a imagePullPolicy as parameter
# and it will update all hard-coded imagePullPolicy
# in the deployments, demos and controller code
#
# Adapted from https://github.com/intel/pmem-csi/

if [[ $# != 1 ]] || [[ "$1" == "?" ]] || [[ "$1" == "--help" ]] ||
   [[ ! $1 =~ IfNotPresent|Always ]]; then
    echo "Usage: $0 <IfNotPresent|Always>" >&2
    exit 1
fi

IMAGE_PULL_POLICY=$1

echo IMAGE_PULL_POLICY=$IMAGE_PULL_POLICY

sed -i -e "s;\(imagePullPolicy\:\ \).*;\1$IMAGE_PULL_POLICY;" $(git grep -l 'imagePullPolicy' deployments/*.yaml demo/*.yaml)

sed -i -e "s;\(ImagePullPolicy\:\ \).*;\1\"$IMAGE_PULL_POLICY\",;" $(git grep -l 'ImagePullPolicy' pkg/controllers/*/*.go)
