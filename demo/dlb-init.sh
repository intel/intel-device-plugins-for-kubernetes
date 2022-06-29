#!/bin/sh -eu

enable_and_configure_vfs() {
  devpath=$1
  
  sriov_numvfs_path="$devpath/sriov_numvfs"
  if ! test -w "$sriov_numvfs_path"; then
    echo "error: $sriov_numvfs_path is not found or not writable. Check if dlb driver module is loaded"
    exit 1
  fi
  if [ "$(cat "$sriov_numvfs_path")" -ne 0 ]; then
    echo "$devpath already configured"
    exit 0
  fi

  # enable sriov
  echo -n 1 > "$sriov_numvfs_path"

  # configure vf
  # unbind vf
  vf_pciid=$(basename "$(realpath "$devpath/virtfn0")")
  dlb_pci_driver_path=/sys/bus/pci/drivers/dlb2
  echo -n "$vf_pciid" > $dlb_pci_driver_path/unbind

  # assign resources to vf
  vf_resources_path="$devpath/vf0_resources"
  echo -n 2048 > "$vf_resources_path/num_atomic_inflights"
  echo -n 2048 > "$vf_resources_path/num_dir_credits"
  echo -n 8 > "$vf_resources_path/num_dir_ports"
  echo -n 2048 > "$vf_resources_path/num_hist_list_entries"
  echo -n 8192 > "$vf_resources_path/num_ldb_credits"
  echo -n 4 > "$vf_resources_path/num_ldb_ports"
  echo -n 32 > "$vf_resources_path/num_ldb_queues"
  echo -n 32 > "$vf_resources_path/num_sched_domains"
  echo -n 2 > "$vf_resources_path/num_sn0_slots"
  echo -n 2 > "$vf_resources_path/num_sn1_slots"
  # bind vf back to dlb2 driver
  echo -n "$vf_pciid" > $dlb_pci_driver_path/bind

  echo "$devpath configured"
  # TODO: Due to unknown e2e-dlb (Simics) limitations, it is not possible to configure per VF resources based on values
  # reported in /sys/bus/pci/devices/<dev_name>/total_resources/<resource_name>. Therefore, only one VF with 
  # values known to work is enabled. This will be improved in the future to make the scrip more meaningful for real-world
  # deployments (see #1145).
}

# use first dlb device to configure a vf
DEVPATH=$(realpath /sys/bus/pci/drivers/dlb2/????:??:??\.0 | head -1)
enable_and_configure_vfs "$DEVPATH"
