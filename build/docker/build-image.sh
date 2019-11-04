#!/bin/sh -e

IMG=$1
BUILDER=$2

DOCKERFILE="$(dirname $0)/$(basename ${IMG}).Dockerfile"

if [ -z "$IMG" ]; then
    (>&2 echo "Usage: $0 <Dockerfile>")
    exit 1
fi

if [ ! -e "${DOCKERFILE}" ]; then
    (>&2 echo "File ${DOCKERFILE} doesn't exist")
    exit 1
fi

TAG=${TAG:-devel}

BUILD_ARGS=
if [ -d $(dirname $0)/../../vendor ] ; then
    echo "Building images with vendored code"
    BUILD_ARGS="--build-arg DIR=/go/src/github.com/intel/intel-device-plugins-for-kubernetes --build-arg GO111MODULE=off"
fi

if [ -z "${BUILDER}" -o "${BUILDER}" = 'docker' ] ; then
    docker build --pull -t ${IMG}:${TAG} ${BUILD_ARGS} -f ${DOCKERFILE} .
elif [ "${BUILDER}" = 'buildah' ] ; then
    buildah bud --pull-always -t ${IMG}:${TAG} ${BUILD_ARGS} -f ${DOCKERFILE} .
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi
