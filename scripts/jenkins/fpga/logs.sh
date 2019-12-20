#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Show all logs related with fpga.

set -o pipefail
set -o xtrace
set -o errexit

kubectl logs $(kubectl get pods | grep -i test-fpga-region | awk '{print $1}')