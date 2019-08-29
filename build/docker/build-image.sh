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
SRCREV=$(git rev-parse HEAD)

if [ -z "${BUILDER}" -o "${BUILDER}" = 'docker' ] ; then
    docker build --pull -t ${IMG}:${TAG} -f ${DOCKERFILE} .
    docker tag ${IMG}:${TAG} ${IMG}:${SRCREV}
elif [ "${BUILDER}" = 'buildah' ] ; then
    buildah bud --pull-always -t ${IMG}:${TAG} -f ${DOCKERFILE} .
    buildah tag ${IMG}:${TAG} ${IMG}:${SRCREV}
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi
