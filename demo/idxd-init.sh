#!/usr/bin/env bash

set -uo pipefail

DEV="${IDXD_DEVICE_TYPE:-dsa}"
NODE_NAME="${NODE_NAME:-}"

function cmd() {

    echo "$@"

    "${@}"
}

for i in $(accel-config list | jq '.[].dev' | grep "$DEV" | sed 's/\"//g'); do

    cmd accel-config disable-device "$i"

done

ndev=$(accel-config list --idle | jq '.[].dev' | grep -c "$DEV")
nwq=4

for (( i = 0; i < ndev; i++ )); do

    dev="$DEV${i}"

    config="$DEV.conf"

    [ -f "conf/$DEV.conf" ] && config="conf/$DEV.conf"

    [ -f "conf/$DEV-$NODE_NAME.conf" ] && config="conf/$DEV-$NODE_NAME.conf"

    sed "s/X/${i}/g" < "$config" > $dev.conf

    cmd accel-config load-config -c "$dev.conf"

    cmd accel-config enable-device "$dev"

    for (( j = 0; j < nwq; j++ )); do

        wq="$dev/wq${i}.${j}"

        cmd accel-config enable-wq "$wq"

    done

done

exit 0

