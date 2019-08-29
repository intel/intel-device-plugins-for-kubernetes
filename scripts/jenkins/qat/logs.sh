#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Show all logs related with qat.

set -o pipefail
set -o xtrace
set -o errexit

PODS=$(kubectl get pods | grep -i qat | awk '{print $1}')
for pod in $PODS; do
  kubectl logs $pod
done
