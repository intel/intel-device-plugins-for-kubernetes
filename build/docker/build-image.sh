#!/bin/sh -e

IMG=$1
BUILDER=$2

DOCKERFILE="$(dirname $0)/${IMG}.Dockerfile"

if [ -z "$IMG" ]; then
    (>&2 echo "Usage: $0 <Dockerfile>")
    exit 1
fi

if [ ! -e "${DOCKERFILE}" ]; then
    (>&2 echo "File ${DOCKERFILE} doesn't exist")
    exit 1
fi

TAG=$(git rev-parse HEAD)

if [ -z "${BUILDER}" -o "${BUILDER}" = 'docker' ] ; then
    docker build -t ${IMG}:${TAG} -f ${DOCKERFILE} .
    docker tag ${IMG}:${TAG} ${IMG}:devel
elif [ "${BUILDER}" = 'buildah' ] ; then
    buildah bud -t ${IMG}:${TAG} -f ${DOCKERFILE} .
    buildah tag ${IMG}:${TAG} ${IMG}:devel
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi
