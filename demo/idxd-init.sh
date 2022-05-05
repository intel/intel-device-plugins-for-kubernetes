#!/usr/bin/env bash

set -euo pipefail

DEV="${IDXD_DEVICE_TYPE:-dsa}"
NODE_NAME="${NODE_NAME:-}"

function cmd() {

    echo "$@"

    "${@}"
}

for i in $(accel-config list | jq '.[].dev' | grep "$DEV" | sed 's/\"//g'); do

    cmd accel-config disable-device "$i"

done

for i in $(accel-config list --idle | jq '.[].dev' | sed -ne "s/\"$DEV\([0-9]\+\)\"/\1/p"); do

    dev="$DEV${i}"

    config="$DEV.conf"

    [ -f "conf/$DEV.conf" ] && config="conf/$DEV.conf"

    [ -f "conf/$DEV-$NODE_NAME.conf" ] && config="conf/$DEV-$NODE_NAME.conf"

    sed "s/X/${i}/g" < "$config" > scratch/"$dev.conf"

    cmd accel-config load-config -e -c scratch/"$dev.conf"

done
