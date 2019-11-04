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

if [ -z "$BUILDER" -o "$BUILDER" = 'docker' ] ; then
    docker build --pull -t ${IMG}:${TAG} "$CWD/$DIR/"
elif [ "$BUILDER" = 'buildah' ] ; then
    buildah bud  --pull-always -t ${IMG}:${TAG} "$CWD/$DIR/"
else
    (>&2 echo "Unknown builder $BUILDER")
    exit 1
fi
