#!/bin/bash
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Copy the licenses of ".Deps" modules for a package to a target directory

set -o errexit
set -o nounset
set -o pipefail

if [ $# != 2 ] || [ "$1" = "?" ] || [ "$1" = "--help" ]; then
	echo "Usage: $0 <package> <license target dir>" >&2
	exit 1
fi

if [ ! -d $2 ] || [ ! -w $2 ]; then
	echo "Error: cannot use $2 as the license target directory"
	exit 1
fi

export GO111MODULE=on

if [ ! -d vendor ]; then
	go mod vendor -v
fi

LICENSE_FILES=$(find vendor |grep -e LICENSE -e NOTICE|cut -d / -f 2-)
PACKAGE_DEPS=$(go list -f '{{ join .Deps "\n" }}' $1 |grep "\.")

pushd vendor > /dev/null

for lic in $LICENSE_FILES; do
	# Copy the license if its repository path is found in package .Deps
	if [ $(echo $PACKAGE_DEPS | grep -c `dirname $lic`) -gt 0 ]; then
		cp -t $2 --parent $lic
	fi
done

popd > /dev/null
