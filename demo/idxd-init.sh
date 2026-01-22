#!/usr/bin/env bash

set -euo pipefail

DEV="${DEVICE_TYPE:-dsa}"
DSA_DRIVER=${DSA_DRIVER:-idxd}
NODE_NAME="${NODE_NAME:-}"
DSA_PCI_IDS=${DSA_PCI_IDS:-0b25 11fb 1212}
OPT=""
[ "$DEV" != "dsa" ] && OPT="-v"

function cmd() {

    echo "$@"

    "${@}"
}

function bind_driver() {
    NEW_DRIVER=$1
    DSA_PFS=$2

    DEVS=""
    for PCIDEV in $(realpath /sys/bus/pci/devices/*); do
        for PF in $DSA_PFS; do
            if grep -q "$PF" "$PCIDEV"/device; then
                DEVS="$PCIDEV $DEVS"
            fi
        done
    done

    for D in $DEVS; do
        BSF=$(basename "$D")
        if [ -e "$D/driver" ]; then
            P=$(realpath -L "$D/driver")
            DRIVER=$(basename "$P")
        else
            DRIVER=""
        fi

        if [ "$DRIVER" != "$NEW_DRIVER" ]; then
            if [ -n "$DRIVER" ]; then
                echo -n "$BSF" >/sys/bus/pci/drivers/"$DRIVER"/unbind
            fi
            echo -n "$NEW_DRIVER" >/sys/bus/pci/devices/"$BSF"/driver_override
            echo -n "$BSF" >/sys/bus/pci/drivers/"$NEW_DRIVER"/bind
        fi
    done
}

[[ "$DEV" == "dsa" && -e /sys/bus/pci/drivers/"$DSA_DRIVER" ]] && bind_driver "$DSA_DRIVER" "$DSA_PCI_IDS"

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
