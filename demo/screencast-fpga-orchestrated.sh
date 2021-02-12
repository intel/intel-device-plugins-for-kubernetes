#!/bin/bash -e

PV='pv -qL'

# check if FPGA devices exist
if [ -d /sys/class/fpga ] ; then
    DRIVER=OPAE
    DEVICE_PREFIX=/dev/intel-fpga-port
elif [ -d /sys/class/fpga_region ] ; then
    DRIVER=DFL
    DEVICE_PREFIX=/dev/dfl-port
else
    >&2 echo 'ERROR: FPGA devices not found or kernel drivers not loaded'
    exit 1
fi

command()
{
  speed=$2
  [ -z "$speed" ] && speed=10

  echo "> $1" | $PV $speed
  sh -c "$1"
  echo | $PV $speed
}

out()
{
  speed=$2
  [ -z "$speed" ] && speed=10

  echo "$1" | $PV $speed
  echo | $PV $speed
}

cleanup()
{
  clear
  out 'Cleanup demo artifacts' 200
  command 'kubectl delete pod test-fpga-orchestrated || true' 200
  command 'kubectl delete -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/deployments/fpga_admissionwebhook/mappings-collection.yaml || true' 200
  command 'kubectl delete namespace intelfpgaplugin-system || true' 200
  command 'kubectl annotate node --all fpga.intel.com/device-plugin-mode-' 200
}


record()
{
  clear
  out 'Record this screencast'
  command "asciinema rec -t 'Intel FPGA Device Plugin for Kubernetes in orchestrated mode with $DRIVER kernel driver.'  Intel-FPGA-Device-Plugin-for-Kubernetes-orchestrated-$DRIVER-Demo.cast -c 'sh ./screencast-fpga-orchestrated.sh play'" 300
}

screen1()
{
  clear
  out "This screencast demonstrates deployment of the Intel FPGA Plugin for Kubernetes in orchestrated mode with $DRIVER kernel driver"
  out "Let's get started!"
  out '1. Check if Kubernetes node is in good shape:'
  command 'kubectl get nodes'
  command 'kubectl get pods -n kube-system'
  sleep 2
  out 'Check if cert-manager is running:'
  command 'kubectl get pods -n cert-manager'
  sleep 2
  out 'Check if CRI-O is running k8s pods:'
  command 'sudo crictl ps -o json | jq .containers[].metadata.name'
  sleep 1
}

screen2()
{
  clear
  out '2. Deploy FPGA plugin'
  command 'kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/fpga_plugin/overlays/region' 100
  sleep 2
  out 'Deploy example mappings:'
  command 'kubectl apply -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/deployments/fpga_admissionwebhook/mappings-collection.yaml' 100
  sleep 2
  out 'Check if the plugin pods are running:'
  command 'kubectl get pods -n intelfpgaplugin-system'
  sleep 2
  out 'Check webhook pod logs:'
  command "kubectl logs $(kubectl get pods -n intelfpgaplugin-system| grep intelfpgaplugin-webhook | awk '{print $1}') -n intelfpgaplugin-system"
  sleep 2
  out 'Check if resource fpga.intel.com/region-<id> is allocatable:'
  command 'kubectl describe node  |grep -A3 Allocatable'
  sleep 2
}

screen3()
{
  clear
  out '3. Install bistreams'
  command 'sudo /opt/intel/fpga-sw/fpgatool -b nlb0.gbs install'
  command 'sudo /opt/intel/fpga-sw/fpgatool -b nlb3.gbs install'
  out 'Create convenience symlinks:'
  command 'cd /srv/intel.com/fpga ; sudo ln -s 69528db6eb31577a8c3668f9faa081f6 Arria10.dcp1.2'
  command "cd /srv/intel.com/fpga/Arria10.dcp1.2 ; sudo ln -s d8*.gbs nlb0.gbs ; sudo ln -s f7*.gbs nlb3.gbs"
  out 'The bitstream storage diretory should look like this:'
  command 'ls -lR /srv/intel.com/fpga/'
  sleep 2
}

screen4()
{
  clear
  out '5. Run OPAE workload that uses NLB3 bitstream'
  out 'Program devices with NLB0 bitstream that is not wanted by the workload:'
  command "sudo /opt/intel/fpga-sw/fpgatool -b /srv/intel.com/fpga/Arria10.dcp1.2/nlb0.gbs -d ${DEVICE_PREFIX}.0 pr"
  command "sudo /opt/intel/fpga-sw/fpgatool -b /srv/intel.com/fpga/Arria10.dcp1.2/nlb0.gbs -d ${DEVICE_PREFIX}.1 pr"
  out 'Check if devices are programmed with it:'
  command 'cat /sys/class/*/*/*/afu_id'
  out 'Run workload:'
  command 'curl https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/demo/test-fpga-orchestrated.yaml' 100
  command 'kubectl create -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/demo/test-fpga-orchestrated.yaml' 100
  sleep 5
  out 'Look at the test output'
  command 'kubectl logs test-fpga-region'
  sleep 2
  out 'Check if orchestration reprogrammed one device with required(NLB3) bitstream:'
  command 'cat /sys/class/*/*/*/afu_id'
  sleep 1
}

screen5()
{
  clear
  out 'Summary:' 15
  out "This screencast demonstrated 'Orchestration programmed' use case for FPGA:" 15
  out ' - FPGA device was programmed by the kubernetes machinery before running the workload' 15
  out ' - desired bitstream resource was specified in the pod spec as fpga.intel.com/arria10.dcp1.2-nlb3-orchestrated' 15
  out ' - the machinery mapped arria10.dcp1.2-nlb3-orchestrated into the pair of region id/AFU id using admission controller webhook' 15
  out ' - programming was done by fpgatool utility installed by the init container into /opt/intel/fpga-sw' 15
  out
  out 'More detailed information about Intel Device Plugins can be found at https://github.com/intel/intel-device-plugins-for-kubernetes' 15
}

if [ "$1" == 'play' ] ; then
  if [ -n "$2" ] ; then
    screen$2
  else
    for n in $(seq 5) ; do screen$n ; sleep 3; done
  fi
elif [ "$1" == 'cleanup' ] ; then
  cleanup
elif [ "$1" == 'record' ] ; then
  record
else
   echo 'Usage: screencast-fpga-orchestrated.sh [--help|help|-h] | [play [<screen number>]] | [cleanup] | [record]'
fi
