#!/bin/bash -e

BUILD_ARGS=
PULL=yes

POSITIONAL=()
while [[ $# -gt 0 ]]; do
    key="$1"

    case $key in
	--build-arg)
	BUILD_ARGS="${BUILD_ARGS} --build-arg=$2"
	shift # past argument
	shift # past value
	;;
	--no-pull)
	PULL=
	shift # past argument
	;;
	*)    # unknown option
	POSITIONAL+=("$1") # save it in an array for later
	shift # past argument
	;;
    esac
done
set -- "${POSITIONAL[@]}" # restore positional parameters

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
    docker build ${PULL/yes/--pull} -t ${IMG}:${TAG} ${BUILD_ARGS} -f ${DOCKERFILE} .
    docker tag ${IMG}:${TAG} ${IMG}:${SRCREV}
elif [ "${BUILDER}" = 'buildah' ] ; then
    buildah bud  ${PULL/yes/--pull-always} -t ${IMG}:${TAG} ${BUILD_ARGS} -f ${DOCKERFILE} .
    buildah tag ${IMG}:${TAG} ${IMG}:${SRCREV}
else
    (>&2 echo "Unknown builder ${BUILDER}")
    exit 1
fi
