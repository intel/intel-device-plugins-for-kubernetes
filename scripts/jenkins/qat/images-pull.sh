#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Pull specific jenkins-build images to test qat-plugin and crypto-perf app
# using crio.

set -o pipefail
set -o errexit

REG=${REG:-intel/}
TAG=${TAG:-devel}

# If registry is a custom one, then append creds.
if [ "$REG" != "intel/" ]; then
  # Verify if user credentials are set.
  if [ -z "$RUSER" ] || [ -z "$RPASS" ]; then
    echo "ERROR: registry user and password is required.";
    exit 1;
  fi

  CREDS="--creds ${RUSER}:${RPASS}"
fi

sudo bash -c "crictl pull ${CREDS} ${REG}intel-qat-plugin:${TAG}"
sudo bash -c "crictl pull ${CREDS} ${REG}crypto-perf:${TAG}"
