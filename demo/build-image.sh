#!/bin/sh -xe

CWD=`dirname $0`
DEVICEDIR=$1
IMG=$2

if [ -z "$DEVICEDIR/$IMG" ]; then
    (>&2 echo "Usage: $0 <device directory> <image directory>")
    exit 1
fi

if [ ! -d "$DEVICEDIR/$IMG" ]; then
    (>&2 echo "Directory $IMG doesn't exist")
    exit 1
fi

if [ -z "$DEVICEDIR" ]; then
    (>&2 echo "Usage: $0 <device directory> <image directory>")
    exit 1
fi

if [ ! -d "$DEVICEDIR" ]; then
    (>&2 echo "Directory $DEVICEDIR doesn't exist")
    exit 1
fi

PROXY_VARS="http_proxy https_proxy"

BUILD_ARGS=""

for proxy in $PROXY_VARS; do
    if [ -v $proxy ]; then
        val=`echo ${!proxy} | tr -d ' '`
        BUILD_ARGS="${BUILD_ARGS} --build-arg $proxy=${val}"
        RUN_ARGS="$RUN_ARGS -e $proxy=${!proxy}"
    fi
done

docker build -t $IMG $BUILD_ARGS "$CWD/$DEVICEDIR/$IMG/"
