#!/bin/sh -xe

WORK_DIR=$1

cd $WORK_DIR

./fft

uprightdiff --format json output.pgm /expected.pgm diff.pgm > diff.json

MODIFIED_AREA=`cat diff.json | jq '.modifiedArea'`

if [ $MODIFIED_AREA -gt 10 ]; then
    echo "The image difference with expected result is too big: ${MODIFIED_AREA} pixels"
    exit 255
fi
echo "Success"
