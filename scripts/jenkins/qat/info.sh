#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Get info from all pods related with qat

set -o pipefail
set -o xtrace
set -o errexit

kubectl get pods
kubectl describe pods