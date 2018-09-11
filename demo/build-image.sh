#!/bin/sh -xe

IMG=$1

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

docker build --rm -t ${IMG}:${TAG} "$CWD/$IMG/"
docker tag ${IMG}:${TAG} ${IMG}:devel
