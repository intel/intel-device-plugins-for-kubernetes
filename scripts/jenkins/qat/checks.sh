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

HPAGES=$(sudo cat /proc/cmdline | grep "hugepages=1024")
if [ -z "$HPAGES" ]; then
 echo "ERROR: no hugepages cmdline flag set."
 ERROR=1
fi

IOMMU=$(sudo cat /proc/cmdline | grep "intel_iommu=on iommu=pt")
if [ -z "$IOMMU" ]; then
 echo "ERROR: no iommu cmdline flags set."
 ERROR=1
fi

QAT=$(sudo swupd bundle-list | grep -i qat)
if [ -z "$QAT" ]; then
 echo "ERROR: linux-qat-firmware bundle is not installed."
 ERROR=1
fi

DPDK=$(sudo swupd bundle-list | grep -i dpdk)
if [ -z "$DPDK" ]; then
 echo "ERROR: dpdk bundle is not installed."
 ERROR=1
fi

VFIO=$(sudo lsmod | grep -i vfio_pci)
if [ -z "$VFIO" ]; then
 echo "ERROR: vfio_pci kernel module is not enabled."
 ERROR=1
fi

PFS=$(for i in 0434 0435 37c8 6f54 19e2; do sudo lspci -D -d 8086:$i; done)
if [ -z "$PFS" ]; then
 echo "ERROR: no qat physical functions were found."
 ERROR=1
fi

VFS=$(for i in 0442 0443 37c9 19e3; do sudo lspci -D -d 8086:$i; done)
if [ -z "$VFS" ]; then
 echo "ERROR: no qat virtual functions were found."
 ERROR=1
fi

if [ -n "$ERROR" ]; then
  exit 1
fi
