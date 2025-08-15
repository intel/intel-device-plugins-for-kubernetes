#!/usr/bin/env bash
NODE_NAME="${NODE_NAME:-}"
ENABLED_QAT_PF_PCIIDS=${ENABLED_QAT_PF_PCIIDS:-37c8 4940 4942 4944 4946 4948}

AUTORESET_ENABLED="NONE"
AUTORESET_ENABLED_FOUND="FALSE"
AUTORESET_OPTIONS_LIST="on off"

DEVS=""
for DEV in $(realpath /sys/bus/pci/devices/*); do
  for PF in $ENABLED_QAT_PF_PCIIDS; do
    if grep -q "$PF" "$DEV"/device; then
      DEVS="$DEV $DEVS"
    fi
  done
done

check_config() {
  [ -f "conf/qat.conf" ] && AUTORESET_ENABLED=$(grep "^AutoresetEnabled=" conf/qat.conf | cut -d= -f 2 | grep '\S')
  [ -f "conf/qat-$NODE_NAME.conf" ] && AUTORESET_ENABLED=$(grep "^AutoresetEnabled=" conf/qat-"$NODE_NAME".conf | cut -d= -f 2 | grep '\S')

  if [ "$AUTORESET_ENABLED" != "NONE" ]; then
    AUTORESET_ENABLED_FOUND="FALSE"
    for OPTION in $AUTORESET_OPTIONS_LIST
    do
      if [ "$OPTION" = "$AUTORESET_ENABLED" ]; then
        AUTORESET_ENABLED_FOUND="TRUE"
        break
      fi
    done
  fi
}

enable_auto_reset() {
  if [ "$AUTORESET_ENABLED_FOUND" = "TRUE" ]; then
    for devpath in $DEVS; do
      autoreset_path="$devpath"/qat/auto_reset
      if ! test -w "$autoreset_path"; then
        echo "error: $autoreset_path is not found or not writable. Check if QAT driver module is loaded. Skipping..."
        exit 1
      fi
      if [ "$(cat "$autoreset_path")" = "$AUTORESET_ENABLED" ]; then
        echo "$devpath's auto reset is already $AUTORESET_ENABLED"
      else
        echo "$AUTORESET_ENABLED" > "$autoreset_path" && echo "$devpath's auto reset has been set $AUTORESET_ENABLED"
      fi
    done
  fi
}

check_config
enable_auto_reset
