#!/bin/sh -xe

WORKSPACE=$1

WORKSPACE="${WORKSPACE:-/tmp}"

cd $WORKSPACE

git clone --depth 1 git://anongit.freedesktop.org/beignet
cd beignet
mkdir build
cd build
cmake ..
make -j4
make -j4 utest
make install
