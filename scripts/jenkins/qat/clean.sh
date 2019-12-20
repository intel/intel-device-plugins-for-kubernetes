#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Clean all artifacts related with qat.

set -o pipefail
set -o xtrace
set -o errexit

kubectl delete daemonset.apps/intel-qat-plugin
kubectl delete configmap intel-qat-plugin-config
kubectl delete pod $(kubectl get pods | grep tc1 | awk '{print $1}')