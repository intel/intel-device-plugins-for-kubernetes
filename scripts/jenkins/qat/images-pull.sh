#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Pull specific jenkins-build images to test qat-plugin and crypto-perf app
# using crio.

set -o pipefail
set -o xtrace
set -o errexit

sudo crictl pull --creds ${RUSER}:${RPASS} ${REG}intel-qat-plugin:${TAG}
sudo crictl pull --creds ${RUSER}:${RPASS} ${REG}crypto-perf:${TAG}
