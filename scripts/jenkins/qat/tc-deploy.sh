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

kubectl apply -k ./deployments/qat_dpdk_app/test-${TCNAME}${TCNUM}/
kubectl wait --for=condition=Initialized pod/qat-dpdk-test-${TACNAME}-perf-tc${TCNUM} --timeout=5m && sleep 60s
kubectl logs -f qat-dpdk-test-crypto-perf-tc1 | tee qat-dpdk-test-${TCNAME}-perf-tc${TCNUM}.log
