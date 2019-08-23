#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Invoke this script on a system that has Docker installed
# such that it can be used by the current user. Then the script
# will bump up the CLEAR_LINUX_BASE and CLEAR_LINUX_VERSION
# parameters in the Dockerfiles such that they pick the
# current version of Clear Linux.
#
# The script is adapted from https://github.com/intel/pmem-csi/

die () {
    echo "ERROR: $@"
    exit 1
}

IMG=$1
shift
DOCKERFILES=$@

docker image pull $IMG || die "pulling $IMG failed"
base=$(docker inspect --format='{{index .RepoDigests 0}}' $IMG) || die "failed to inspect $IMG"
echo "Base image: $base"

# We rely on swupd to determine what this particular image can be
# updated to with "swupd update --version". This might not be the very latest
# Clear Linux, for example when there has been a format bump and the
# base image is still using the older format.
output=$(docker run $base swupd check-update) # will return non-zero exit code if there is nothing to update
# The expected output on failure is one of:
#     Current OS version: 30450
#     Latest server version: 30450
#     There are no updates available
# or:
#     Current OS version: 29940
#     Latest server version: 29970
#     There is a new OS version available: 29970
version=$(echo "$output" | grep "Latest server version" | tail -n 1 | sed -e 's/.*: *//')
if [ ! "$version" ]; then
    die "failed to obtain information about available updates"
fi
echo "Update version: $version"

# Do a trial-run with these parameters.
docker run "$base" swupd update --version=$version || die "failed to update"

# Now update the Dockerfile(s).
sed -i -e 's;^\(ARG CLEAR_LINUX_BASE=\).*;\1'"$base"';' -e 's;^\(ARG CLEAR_LINUX_VERSION=\).*;\1"--version='"$version"'";' $DOCKERFILES || die "failed to patch Dockerfiles"

echo "Done."
