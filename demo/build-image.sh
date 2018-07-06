#!/bin/sh -xe

CWD=`dirname $0`
IMG=$1

if [ -z "$IMG" ]; then
    (>&2 echo "Usage: $0 <image directory>")
    exit 1
fi

if [ ! -d "$IMG" ]; then
    (>&2 echo "Directory $IMG doesn't exist")
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

docker build -t ${IMG} $BUILD_ARGS "$CWD/$IMG/"
