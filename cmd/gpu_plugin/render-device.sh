#!/bin/sh
#
# Copyright 2021-2023 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
#
# Some GPU workloads are unable to find the (Intel) GPU provisioned for
# them by Kubernetes. This script checks and tells which device to use.
#
# For example (all?) media applications using VA-API or QSV media APIs [1],
# fail when /dev/dri/renderD128 is not present, or happens to be of
# a type not supported by the media driver.
#
# Happily (all?) media applications have an option to specify a suitable
# render device name, which can be used with this script.
#
# [1] Compute, 3D, and OneVPL APIs do not suffer from this issue.
#
#
# Running the script requires only few tools, which should be present in
# all distro base images.  The required tools, and the packages they
# reside in Debian based distros, are:
# - dash: 'sh' (minimal bourne shell)
# - coreutils: 'seq', 'cat', 'echo'
# - sed: 'sed'
#
# But they are also provided by 'busybox' and 'toybox' tool sets.


usage ()
{
	name=${0##*/}
	echo "Provides (Intel GPU) render device name application can use, either"
	echo "on standard output, or added to given command line. If device index"
	echo "N is given, provides name of Nth available (Intel GPU) render device."
	echo
	echo "Usage:"
	echo "  $name <device index>"
	echo "  $name [device index] <media program> [other options] <GPU selection option>"
	echo
	echo "Examples:"
	echo "  \$ vainfo --display drm --device \$($name 1)"
	echo "  \$ $name vainfo --display drm --device"
	echo "  Running: vainfo --display drm --device /dev/dri/renderD140"
	echo
	echo "ERROR: $1!"
	exit 1
}

if [ $# -eq 0 ]; then
	usage "no arguments given"
fi

# determine required GPU index
NaN=$(echo "$1" | sed 's/[0-9]\+//')
if [ "$NaN" = "" ] && [ "$1" != "" ]; then
	required=$1
	if [ "$required" -lt 1 ] || [ "$required" -gt 127 ]; then
		usage "GPU index $required not in range 1-127"
	fi
	shift
else
	required=1
fi
visible=0

vendor=""
intel="0x8086"
# find host index "i" for Nth visible Intel GPU device
for i in $(seq 128 255); do
	if [ -w "/dev/dri/renderD$i" ]; then
		vendor=$(cat "/sys/class/drm/renderD$i/device/vendor")
		if [ "$vendor" = "$intel" ]; then
			visible=$((visible+1))
			if [ $visible -eq $required ]; then
				break
			fi
		fi
	fi
done

if [ $visible -ne $required ]; then
	usage "$visible Intel GPU(s) found, not $required as requested"
fi
device="/dev/dri/renderD$i"

if [ $# -eq 0 ]; then
	echo "$device"
	exit 0
fi

if [ $# -lt 2 ]; then
	usage "media program and/or GPU selection option missing"
fi

# run given media workload with GPU device name appended to end
echo "Running: $* $device"
exec "$@" "$device"
