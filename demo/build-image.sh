#!/bin/sh -e

IMG=$1
BUILDER=$2

if [ -z "$IMG" ]; then
    (>&2 echo "Usage: $0 <image directory>")
    exit 1
fi

if [ ! -d "$IMG" ]; then
    (>&2 echo "Directory $IMG doesn't exist")
    exit 1
fi

CWD=`dirname $0`
TAG=`git rev-parse HEAD`

if [ -z "$BUILDER" -o "$BUILDER" = 'docker' ] ; then
    docker build --rm -t ${IMG}:${TAG} "$CWD/$IMG/"
    docker tag ${IMG}:${TAG} ${IMG}:devel
elif [ "$BUILDER" = 'buildah' ] ; then
    buildah bud -t ${IMG}:${TAG} "$CWD/$IMG/"
    buildah tag ${IMG}:${TAG} ${IMG}:devel
else
    (>&2 echo "Unknown builder $BUILDER")
    exit 1
fi
