#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Invoke this script with a version/tag and/or an org
# as parameter and it will update all hard-coded image
# versions and/or orgs in the source code.
#
# Adapted from https://github.com/intel/pmem-csi/

OPTS=`getopt -o ht:o: --long help,tag:,org: -n 'test' -- "$@"`

if [ $? != 0 ]; then
    echo "Failed parsing options." >&2
    exit 1
fi

while true; do
  case "${1}" in
    -h | --help) usage="true"; shift ;;
    -o | --org ) org=$2; shift 2;;
    -t | --tag ) tag=$2; shift 2;;
    -- ) shift; break ;;
    * )  break ;;
  esac
done

if [ "${usage}" ]; then
  cat <<-EOF
  Set custom versions and/or custom orgs

  Usage:

    set-version.sh [arguments]

  Arguments:

    -t or --tag     'Set a custom image version/tag'           Example: 'set-version.sh -t v1.1.0'
    -o or --org     'Set a custom registry organization'       Example: 'set-version.sh -o docker.io'
    -h or --help    'Show this'
	EOF
  exit 0;
fi

cd $(realpath $(dirname $0)/..)

if [ "$tag" ]; then
  sed -i -e "s;\(^TAG?*=\|.*/crypto-perf:\|.*/opae-nlb-demo:\|.*/intel-[^ ]*:\)[^ ]*;\1$tag;g" $(git grep -l '^TAG?*=\|.*/crypto-perf:\|.*/opae-nlb-demo:\|.*/intel-[^ ]*:' Makefile deployments demo/*fpga*.yaml)
fi

if [ "$org" ]; then
  sed -i -e "s;\(^ORG?*=\)[^ ]*;\1$org;g" $(git grep -l '^ORG?*=\|.*/crypto-perf:\|.*/opae-nlb-demo:\|.*/intel-[^ ]*:' Makefile deployments demo/*fpga*.yaml)
  sed -i -e "s;\(.*image:[ ]*\).*\(/\(intel\|crypto-perf\|opae-nlb-demo\)[^ ]*\);\1$org\2;g" $(git grep -l '^ORG?*=\|.*/crypto-perf:\|.*/opae-nlb-demo:\|.*/intel-[^ ]*:' Makefile deployments demo/*fpga*.yaml)
fi
