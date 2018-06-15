#!/bin/sh -e

# Test FPGA by running 2 OPAE samples: nlb3 and nlb0
# nlb3 is expected to succeed, nlb0 - to fail
green () { echo "\033[0;32m$1\033[0m";}
red () { echo "\033[0;31m$1\033[0m";}

green 'Running nlb3'
if nlb3; then
    green 'nlb3 succeeded as expected'
    green 'Running nlb0 sample'
    if nlb0; then
        red 'nlb0 succeeded'
        red 'FAILURE: unexpected nlb0 success'
    else
        green 'nlb0 failed as expected'
        green 'SUCCESS'
    fi
else
    red 'nlb3 failed'
    green 'FAILURE: unexpeced nlb3 failure'
fi
