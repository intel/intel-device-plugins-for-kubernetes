#!/usr/bin/env bash
# This script is based on qatlib's qat_init.sh
NODE_NAME="${NODE_NAME:-}"
ENABLED_QAT_PF_PCIIDS=${ENABLED_QAT_PF_PCIIDS:-37c8 4940 4942 4944 4946 4948}
# TODO: check against qatlib
SERVICES_LIST="sym asym sym;asym dc sym;dc asym;dc dcc decomp asym;sym;dc asym;sym;decomp"
QAT_4XXX_DEVICE_PCI_ID="0x4940"
QAT_401XX_DEVICE_PCI_ID="0x4942"
QAT_402XX_DEVICE_PCI_ID="0x4944"
QAT_420XX_DEVICE_PCI_ID="0x4946"
QAT_6XXX_DEVICE_PCI_ID="0x4948"
SERVICES_ENABLED="NONE"
SERVICES_ENABLED_FOUND="FALSE"

DEVS=""
for DEV in $(realpath /sys/bus/pci/devices/*); do
  for PF in $ENABLED_QAT_PF_PCIIDS; do
    if grep -q "$PF" "$DEV"/device; then
      DEVS="$DEV $DEVS"
    fi
  done
done

check_config() {
  [ -f "conf/qat.conf" ] && SERVICES_ENABLED=$(grep "^ServicesEnabled=" conf/qat.conf | cut -d= -f 2 | grep '\S')
  [ -f "conf/qat-$NODE_NAME.conf" ] && SERVICES_ENABLED=$(grep "^ServicesEnabled=" conf/qat-"$NODE_NAME".conf | cut -d= -f 2 | grep '\S')

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
    for DEVPATH in $DEVS; do
      PCI_DEV=$(cat "$DEVPATH"/device 2> /dev/null)
      if [ "$PCI_DEV" != "$QAT_4XXX_DEVICE_PCI_ID" ] && [ "$PCI_DEV" != "$QAT_401XX_DEVICE_PCI_ID" ] && [ "$PCI_DEV" != "$QAT_402XX_DEVICE_PCI_ID" ] && [ "$PCI_DEV" != "$QAT_420XX_DEVICE_PCI_ID" ] && [ "$PCI_DEV" != "$QAT_6XXX_DEVICE_PCI_ID" ]; then
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
      echo "Device $DEVPATH configured with services: $CURRENT_SERVICES"
    done
  fi
}

enable_sriov() {
  for DEVPATH in $DEVS; do
  NUMVFS="$DEVPATH"/sriov_numvfs
  if ! test -w "$NUMVFS"; then
    echo "error: $NUMVFS is not found or not writable. Check if QAT driver module is loaded"
    exit 1
  fi
  if ! test -d /sys/bus/pci/drivers/vfio-pci; then
    echo "error: vfio-pci driver needed by QAT VFs must be loaded"
    exit 1
  fi
  if [ "$(cat "$NUMVFS")" -ne 0 ]; then
    echo "$DEVPATH already configured"
  else
    tee "$NUMVFS" < "$DEVPATH"/sriov_totalvfs
    VFDEVS=$(realpath -L "$DEVPATH"/virtfn*)
    for vfdev in $VFDEVS; do
      BSF=$(basename "$vfdev")
      VF_DEV="/sys/bus/pci/devices/$BSF"
      if test -e "$VF_DEV/driver"; then
        P=$(realpath -L "$VF_DEV/driver")
        VF_DRIVER=$(basename "$P")
      else
        VF_DRIVER=""
      fi
      if [ "$VF_DRIVER" != "vfio-pci" ]; then
        if [ "$VF_DRIVER" ]; then
          echo -n "$BSF" > /sys/bus/pci/drivers/"$VF_DRIVER"/unbind
        fi
        echo -n vfio-pci > /sys/bus/pci/devices/"$BSF"/driver_override
        echo -n "$BSF" > /sys/bus/pci/drivers/vfio-pci/bind
      fi
    done
  fi
  done
}

check_config
sysfs_config
enable_sriov
