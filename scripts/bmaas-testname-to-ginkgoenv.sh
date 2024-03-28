#!/bin/bash

set -e

name=$1
shift

unset FOCUS
unset SKIP

case $name in
  e2e-qat)
    export FOCUS="Mode:dpdk.*Resource:(cy|dc)"
    export SKIP="App:crypto-perf"
    ;;

  e2e-gpu)
    export SKIP="Resource:xe"
    ;;

  e2e-sgx)
    export FOCUS="|(SGX Admission)"
    ;;
esac
