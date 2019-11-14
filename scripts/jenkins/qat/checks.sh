#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Checks if all pre-requirements for qat-dpdk are up.

set -o pipefail
set -o xtrace
set -o errexit

sudo dmesg | grep -i qat
sudo cat /proc/cmdline

if [ 0 -eq $(ls -1 /sys/class/iommu|wc -l) ]; then
 echo "ERROR: no iommu con flags set."
 ERROR=1
fi

if [ 0 -eq $(sudo swupd bundle-list |grep -i qat |wc -l) ]; then
 echo "ERROR: linux-qat-firmware bundle is not installed."
 ERROR=1
fi

if [ 0 -eq $(lsmod |grep -i vfio_pci| wc -l) ]; then
 echo "ERROR: vfio_pci kernel module is not enabled."
 ERROR=1
fi

if [ 0 -eq $(for i in 0442 0443 37c9 19e3; do lspci -D -d 8086:$i; done|wc -l) ]; then
 echo "ERROR: no qat virtual functions were found."
 ERROR=1
fi

if [ -n "$ERROR" ]; then
  exit 1
else
  echo "OK: all checks pass."
fi
