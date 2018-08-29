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

docker build -t ${IMG} "$CWD/$IMG/"
