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
    docker build --pull -t ${IMG}:${TAG} -f ${DOCKERFILE} .
    # If $CUSTOM_TAG is set as an env var, then the final tag,
    # is replaced with $CUSTOM_TAG value, otherwise "devel",
    # string constant is set.
    CUSTOM_TAG=${CUSTOM_TAG:-"devel"}
    # If custom $CUSTOM_REGISTRY is defined as an env var,
    # then the image name is prefixed with the registry name,
    # if not registry provided, $CUSTOM_NAME defaults with,
    # $IMG value.
    CUSTOM_NAME=${IMG}
    if [ -n "$CUSTOM_REGISTRY" ]; then
      CUSTOM_NAME="$CUSTOM_REGISTRY/$IMG"
    fi
    # TODO: check if this mechanism of tagging applies only,
    # for Docker builds.
    docker tag ${IMG}:${TAG} ${CUSTOM_NAME}:${CUSTOM_TAG}
elif [ "${BUILDER}" = 'buildah' ] ; then
    buildah bud --pull-always -t ${IMG}:${TAG} -f ${DOCKERFILE} .
    buildah tag ${IMG}:${TAG} ${IMG}:devel
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi
