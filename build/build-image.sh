#!/bin/sh -e

IMG=$1

REPO=$(basename ${IMG})
ROOT="$(dirname $0)/../"
DOCKERFILE="$REPO.Dockerfile"

if [ -z "$IMG" ]; then
    (>&2 echo "Usage: $0 <Dockerfile>")
    exit 1
fi

if [ ! -e "${DOCKERFILE}" ]; then
    (>&2 echo "File ${DOCKERFILE} doesn't exist")
    exit 1
fi

shift

if [ "$1" = 'docker' -o "$1" = 'buildah' ]; then
    BUILDER=$1
    shift
fi

# If there is a directory at the same path, which has the same name as
# the dockerfile except the suffix, use that directory as the build context,
# otherwise use the repository root as the build context.
if [ -d "./$REPO" ]; then
    CONTEXT="./$REPO"
else
    CONTEXT="$ROOT"
fi

TAG=${TAG:-devel}

BUILD_ARGS=$@
if [ -d "$ROOT/vendor" ] ; then
    echo "Building images with vendored code"
    BUILD_ARGS="${BUILD_ARGS} --build-arg DIR=/go/src/github.com/intel/intel-device-plugins-for-kubernetes --build-arg GO111MODULE=off"
fi

BUILD_ARGS="${BUILD_ARGS} --build-arg FINAL_BASE=gcr.io/distroless/static"
if [ -z "${BUILDER}" -o "${BUILDER}" = 'docker' ] ; then
    docker build --pull -t ${IMG}:${TAG} ${BUILD_ARGS} -f ${DOCKERFILE} ${CONTEXT}
elif [ "${BUILDER}" = 'buildah' ] ; then
    BUILDAH_RUNTIME=runc buildah bud --pull-always -t ${IMG}:${TAG} ${BUILD_ARGS} -f ${DOCKERFILE} ${CONTEXT}
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi
