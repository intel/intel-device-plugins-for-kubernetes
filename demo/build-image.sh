#!/bin/sh -e

IMG=$1
BUILDER=$2
DIR=$(basename $IMG)

if [ -z "$DIR" ]; then
    (>&2 echo "Usage: $0 <image directory>")
    exit 1
fi

if [ ! -d "$DIR" ]; then
    (>&2 echo "Directory $DIR doesn't exist")
    exit 1
fi

CWD=`dirname $0`
TAG=${TAG:-devel}
SRCREV=$(git rev-parse HEAD)

if [ -z "$BUILDER" -o "$BUILDER" = 'docker' ] ; then
    docker build --pull -t ${IMG}:${TAG} "$CWD/$DIR/"
    docker tag ${IMG}:${TAG} ${IMG}:${SRCREV}
elif [ "$BUILDER" = 'buildah' ] ; then
    buildah bud  --pull-always -t ${IMG}:${TAG} "$CWD/$DIR/"
    buildah tag ${IMG}:${TAG} ${IMG}:${SRCREV}
else
    (>&2 echo "Unknown builder $BUILDER")
    exit 1
fi
