#!/usr/bin/env bash

set -euo pipefail

DEV="${DEVICE_TYPE:-dsa}"
NODE_NAME="${NODE_NAME:-}"
OPT=""
[ "$DEV" != "dsa" ] && OPT="-v"

function cmd() {

    echo "$@"

    "${@}"
}

for i in $(accel-config list | jq -r '.[].dev' | grep ${OPT} "dsa"); do

    cmd accel-config disable-device "$i"

done

for i in $(accel-config list --idle | jq -r '.[].dev' | grep ${OPT} "dsa" | sed -e 's/.*\([0-9]\+\)/\1/'); do

    config="$DEV.conf"

    [ -f "conf/$DEV.conf" ] && config="conf/$DEV.conf"

    [ -f "conf/$DEV-$NODE_NAME.conf" ] && config="conf/$DEV-$NODE_NAME.conf"

    sed "s/X/${i}/g" < "$config" > scratch/"$DEV${i}.conf"

    cmd accel-config load-config -e -c scratch/"$DEV${i}.conf"

done
