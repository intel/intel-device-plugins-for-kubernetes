#!/bin/sh -xe

TESTS_DIR=$1

cd $TESTS_DIR

. ./setenv.sh
./utest_run
