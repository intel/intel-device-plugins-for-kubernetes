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
elif [ "$BUILDER" = 'buildah' ] ; then
    buildah bud -t ${IMG}:${TAG} "$CWD/$IMG/"
    buildah tag ${IMG}:${TAG} ${IMG}:devel
else
    (>&2 echo "Unknown builder $BUILDER")
    exit 1
fi
