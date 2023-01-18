#!/usr/bin/env bash

set -euo pipefail

cd /usr/local/bin/low-level-api

time for i in $(ls ll_* | grep -v ll_canned_mode_with_data_example); do
    echo $i;
    ./$i;
    echo $?;
done
