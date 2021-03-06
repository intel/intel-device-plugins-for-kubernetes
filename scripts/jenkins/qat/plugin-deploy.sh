#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Deploys current jenkins build test image 'intel-qat-plugin' in the cluster.

set -o pipefail
set -o xtrace
set -o errexit

REPO_ROOT=$(realpath $(dirname $0)/../../..)
kubectl create -f ${REPO_ROOT}/deployments/qat_plugin/qat_plugin_default_configmap.yaml
kubectl create -f ${REPO_ROOT}/deployments/qat_plugin/qat_plugin.yaml
kubectl rollout status ds/intel-qat-plugin --timeout=5m
kubectl wait --for=condition=Ready pod --all --timeout=5m && sleep 60s
