#!/bin/sh
# This script is based on qatlib's qat_init.sh
NODE_NAME="${NODE_NAME:-}"
ENABLED_QAT_PF_PCIIDS=${ENABLED_QAT_PF_PCIIDS:-37c8 4940 4942}
DEVS=$(for pf in $ENABLED_QAT_PF_PCIIDS; do lspci -n | grep -e "$pf" | grep -o -e "^\\S*"; done)
SERVICES_LIST="sym;asym dc"
QAT_4XXX_DEVICE_PCI_ID="0x4940"
QAT_401XX_DEVICE_PCI_ID="0x4942"
SERVICES_ENABLED="NONE"
SERVICES_ENABLED_FOUND="FALSE"

check_config() {
  [ -f "conf/qat.conf" ] && SERVICES_ENABLED=$(cut -d= -f 2 conf/qat.conf | grep '\S')
  [ -f "conf/qat-$NODE_NAME.conf" ] && SERVICES_ENABLED=$(cut -d= -f 2 conf/qat-$NODE_NAME.conf | grep '\S')

  if [ "$SERVICES_ENABLED" != "NONE" ]; then
    SERVICES_ENABLED_FOUND="FALSE"
    for SERVICE in $SERVICES_LIST
    do
      if [ "$SERVICE" = "$SERVICES_ENABLED" ]; then
        SERVICES_ENABLED_FOUND="TRUE"
        break
      fi
    done
  fi
}

sysfs_config() {
  if [ "$SERVICES_ENABLED_FOUND" = "TRUE" ]; then
    for dev in $DEVS; do
      DEVPATH="/sys/bus/pci/devices/0000:$dev"
      PCI_DEV=$(cat "$DEVPATH"/device 2> /dev/null)
      if [ "$PCI_DEV" != "$QAT_4XXX_DEVICE_PCI_ID" ] && [ "$PCI_DEV" != "$QAT_401XX_DEVICE_PCI_ID" ]; then
        continue
      fi

      CURRENT_SERVICES=$(cat "$DEVPATH"/qat/cfg_services)
      if [ "$CURRENT_SERVICES" != "$SERVICES_ENABLED" ]; then
        CURRENT_STATE=$(cat "$DEVPATH"/qat/state)
        if [ "$CURRENT_STATE" = "up" ]; then
          echo down > "$DEVPATH"/qat/state
        fi
        echo "$SERVICES_ENABLED" > "$DEVPATH"/qat/cfg_services
        CURRENT_SERVICES=$(cat "$DEVPATH"/qat/cfg_services)
      fi
      echo "Device $dev configured with services: $CURRENT_SERVICES"
    done
  fi
}

enable_sriov() {
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
}

check_config
sysfs_config
enable_sriov
