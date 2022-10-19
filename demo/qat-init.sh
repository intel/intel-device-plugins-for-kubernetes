#!/bin/sh -eu
ENABLED_QAT_PF_PCIIDS=${ENABLED_QAT_PF_PCIIDS:-37c8 4940 4942}
DEVS=$(for pf in $ENABLED_QAT_PF_PCIIDS; do lspci -n | grep -e "$pf" | grep -o -e "^\\S*"; done)

for dev in $DEVS; do
  DEVPATH="/sys/bus/pci/devices/0000:$dev"
  NUMVFS="$DEVPATH/sriov_numvfs"
  if ! test -w "$NUMVFS"; then
    echo "error: $NUMVFS is not found or not writable. Check if QAT driver module is loaded"
    exit 1
  fi
  if [ "$(cat "$NUMVFS")" -ne 0 ]; then
    echo "$DEVPATH already configured"
  else
    tee "$NUMVFS" < "$DEVPATH/sriov_totalvfs"
  fi
done
