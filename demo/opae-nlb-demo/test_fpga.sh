#!/bin/bash

# Test FPGA by running 2 OPAE samples: nlb3 and nlb0
# nlb3 is expected to succeed, nlb0 - to fail
green () { echo -e "\033[0;32m$1\033[0m";}
red () { echo -e "\033[0;31m$1\033[0m";}

green 'Running nlb3'
nlb3 && {
    green 'nlb3 succeeded as expected'
    green 'Running nlb0 sample'

    nlb0 && {
        red 'nlb0 succeeded'
        red 'FAILURE: unexpected nlb0 success'

        exit 1
    } || {
        green 'nlb0 failed as expected'
        green 'SUCCESS'

        exit 0
    }
} || {
    red 'nlb3 failed'
    red 'FAILURE: unexpeced nlb3 failure'

    exit 1
}
