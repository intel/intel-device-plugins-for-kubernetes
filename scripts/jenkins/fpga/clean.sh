#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Clean all artifacts related with fpga.

set -o pipefail
set -o xtrace
set -o errexit

kubectl delete pod test-fpga-region
kubectl delete ServiceAccount intel-fpga-plugin-controller --namespace kube-system
kubectl delete ClusterRole node-getter --namespace kube-system
kubectl delete ClusterRoleBinding get-nodes --namespace kube-system
kubectl annotate node --all fpga.intel.com/device-plugin-mode-
kubectl delete daemonset intel-fpga-plugin --namespace kube-system
kubectl delete deployment intel-fpga-webhook-deployment --namespace kube-system