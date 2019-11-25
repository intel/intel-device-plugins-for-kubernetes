#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Deploys current jenkins-build test image for dpdk-app (crypto-perf)
# in the cluster.

set -o pipefail
set -o xtrace
set -o errexit

REPO_ROOT=$(realpath $(dirname $0)/../../..)
for test in $(ls ${REPO_ROOT}/deployments/qat_dpdk_app/ | grep test-*); do
  tname=$(echo $test |sed -e "s;\(test-[a-z]*\)\([0-9]*\);\1;g")
  tnum=$(echo $test|sed -e "s;\(test-[a-z]*\)\([0-9]*\);\2;g")
  kubectl apply -k ${REPO_ROOT}/deployments/qat_dpdk_app/${test}/ && \
  kubectl wait --for=condition=Ready pod/qat-dpdk-${tname}-perf-tc${tnum} --timeout=5m
done
