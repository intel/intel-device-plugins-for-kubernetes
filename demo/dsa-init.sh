#!/usr/bin/env bash

set -euo pipefail

ndev=$(accel-config list --idle | jq '.[].dev' | grep -c dsa)
nwq=4

function cmd() {

    echo "$@"

    "${@}"
}

for (( i = 0; i < ndev; i++ )); do

    dev="dsa${i}"

    sed "s/X/${i}/g" < dsa.conf > $dev.conf

    cmd accel-config load-config -c "$dev.conf"

    cmd accel-config enable-device "$dev"

    for (( j = 0; j < nwq; j++ )); do

        wq="$dev/wq${i}.${j}"

        cmd accel-config enable-wq "$wq"

    done

done
