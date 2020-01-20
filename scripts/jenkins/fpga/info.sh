#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Get info from all pods related with fpga
# for the default and kube-system namespaces

set -o pipefail
set -o xtrace
set -o errexit

kubectl get pods
kubectl describe pods
kubectl get pods --namespace kube-system
kubectl describe pods --namespace kube-system