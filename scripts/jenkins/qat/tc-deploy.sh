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
for test in $(ls -d ${REPO_ROOT}/deployments/qat_dpdk_app/test-*); do
  kubectl apply -k $test &
done
wait
kubectl wait --for=condition=Ready pod --all --timeout=5m && sleep 60s
