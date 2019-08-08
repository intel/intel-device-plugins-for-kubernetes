#!/bin/sh -e

IMG=$1

if [ -z "$IMG" ]; then
    (>&2 echo "Usage: $0 <Dockerfile>")
    exit 1
fi

if [ -z "$CUSTOM_REGISTRY" ]; then
    (>&2 echo "Usage: $0 <Registry>")
    exit 1
fi

# If $CUSTOM_TAG is set as an env var, then the final tag,
# is replaced with $CUSTOM_TAG value, otherwise "devel",
# string constant is set.
CUSTOM_TAG=${CUSTOM_TAG:-"devel"}
# Image name is required to be prefixed with registry name.
# appending its value,
CUSTOM_NAME="$CUSTOM_REGISTRY/$IMG"
# TODO: assuming Docker login to the custom registry was
# executed prior to push.
docker push ${CUSTOM_NAME}:${CUSTOM_TAG}
