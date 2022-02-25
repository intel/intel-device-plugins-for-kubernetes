#!/bin/sh -e
#
# Copyright 2022 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#

die () {
    echo "ERROR: $*"
    exit 1
}

IMG=$1
shift

if [ "$1" = 'docker' ] || [ "$1" = 'buildah' ]; then
    BUILDER=$1
fi

echo "Testing $IMG base layer"

if [ -z "${BUILDER}" ] || [ "${BUILDER}" = 'docker' ] ; then
    distroless_base=$(docker inspect --format='{{index .RootFS.Layers 0}}' "gcr.io/distroless/static") || die "failed to inspect gcr.io/distroless/static"
    img_base=$(docker inspect --format='{{index .RootFS.Layers 0}}' "$IMG") || die "failed to inspect $IMG"
elif [ "${BUILDER}" = 'buildah' ] ; then
    distroless_base=$(buildah inspect --type image --format='{{index .OCIv1.RootFS.DiffIDs 0}}' "gcr.io/distroless/static") || die "failed to inspect gcr.io/distroless/static"
    img_base=$(buildah inspect --type image --format='{{index .OCIv1.RootFS.DiffIDs 0}}' "$IMG") || die "failed to inspect $IMG"
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi

test "${distroless_base}" = "${img_base}" || die "$IMG base layer differs from gcr.io/distroless/static"
