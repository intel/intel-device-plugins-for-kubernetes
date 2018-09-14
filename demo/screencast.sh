#!/bin/bash

PV='pv -qL'

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
  out 'Cleanup demo artifacts' 20
  out 'delete test pod:' 20
  command 'kubectl delete pod test-fpga-region' 20
  out 'delete ServiceAccount:' 20
  command 'kubectl delete ServiceAccount intel-fpga-plugin-controller --namespace kube-system' 20
  out 'delete ClusterRole:' 20
  command 'kubectl delete ClusterRole node-getter --namespace kube-system' 20
  out 'delete ClusterRoleBinding:' 20
  command 'kubectl delete ClusterRoleBinding get-nodes --namespace kube-system' 20
  out 'delete node annotation:' 20
  command 'kubectl annotate node --all fpga.intel.com/device-plugin-mode-' 20
  out 'delete plugin daemonset:' 20
  command 'kubectl delete daemonset intel-fpga-plugin --namespace kube-system' 20
  out 'delete webhook deployment:' 20
  command 'kubectl delete deployment intel-fpga-webhook-deployment --namespace kube-system' 20
  out 'delete images' 20
  #docker rmi -f $(docker images |grep "intel\|opae" | awk '{print $3}' | tr "\n" " ") 
  sudo podman rmi -f $(sudo podman images |grep "intel\|opae" | awk '{print $3}' | tr '\n' ' ')
}

record()
{
  clear
  out 'Record this screencast'
  command 'asciinema rec -t "Intel FPGA Device Plugin for Kubernetes"  Intel-FPGA-Device-Plugin-for-Kubernetes-Demo.cast -c "sh ./demo.sh play"'
}

screen1()
{
  clear
  out "This screencast demonstrates deployment of the Intel FPGA Plugin for Kubernetes"
  out "Let's get started!"
  out "1. Check if Kubernetes node is in good shape:"
  command "kubectl get nodes"
  command "kubectl get pods --all-namespaces"
  out "Check if CRI-O is running k8s pods:"
  command "sudo crictl ps"
}

screen2()
{
  clear
  rm -rf $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
  out '2. Clone Intel Device Plugins for Kubernetes repository from github to the $GOPATH/src/github.com/intel directory'
  command "mkdir -p $GOPATH/src/github.com/intel/; cd $GOPATH/src/github.com/intel; git clone https://github.com/intel/intel-device-plugins-for-kubernetes" 15
}

screen3()
{
  clear
  cd $GOPATH/src/github.com/intel
  out "3. Deploy admission controller webhook"
  out "Build intel-fpga-admissionwebhook image:"
  command "cd intel-device-plugins-for-kubernetes; make intel-fpga-admissionwebhook" 15
  out "Import image from docker to CRI-O:"
  command "docker save intel-fpga-admissionwebhook:devel | sudo podman load"
  cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
  out "Deploy the webhook:"
  command "cd scripts ; ./webhook-deploy.sh --namespace kube-system --mode orchestrated; cd ../"
  sleep 2
  out "Check if its pod is running:"
  command "kubectl get pods --namespace kube-system | grep intel-fpga-webhook"
}

screen4()
{
  clear
  cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
  out "4. Deploy FPGA plugin"
  out "Build intel-fpga-plugin image"
  command "make intel-fpga-plugin" 15
  out "Import image from docker to CRI-O:"
  command "docker save intel-fpga-plugin:devel | sudo podman load"
  out "Build intel-fpga-initcontainer image"
  out "NOTE! This image requires 'Acceleration Stack for Runtime' tarball from https://www.intel.com/content/www/us/en/programmable/solutions/acceleration-hub/downloads.html" 15
  out "We'll skip download part as it takes time and simply copy the tarball to the required location and build the image:" 15
  command "cp /srv/demo/a10_gx_pac_ias_1_1_pv_rte_installer.tar.gz deployments/fpga_plugin/" 15
  command "cd deployments/fpga_plugin/ ; ./build-initcontainer-image.sh" 15
  out "Import image from docker to CRI-O:"
  command "docker save intel-fpga-initcontainer:devel | sudo podman load"
  out "Check that both images are imported:"
  command "sudo crictl images|grep 'intel-fpga-\(i\|p\)'"
  out "Create a service account for the plugin"
  command "kubectl create -f deployments/fpga_plugin/fpga_plugin_service_account.yaml"
  out "Set region mode for the plugin:"
  command "kubectl annotate node --all 'fpga.intel.com/device-plugin-mode=region'"
  out "Create plugin daemonset:"
  command "kubectl create -f deployments/fpga_plugin/fpga_plugin.yaml"
  out "Check if its pod is runnning:"
  command "kubectl get pod --namespace kube-system |grep intel-fpga-plugin"
  out "Check if it runs in 'region' mode:"
  command "kubectl logs $(kubectl  get pods --namespace kube-system |grep intel-fpga-plugin|cut -f1 -d' ') --namespace kube-system"
  out "Check if resource fpga.intel.com/region-<FPGA interface id> is allocatable:"
  command "kubectl describe node  |grep -A5 Allocatable"
}

screen5()
{
  clear
  cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
  out "5. Run OPAE workload that uses NLB3 bitstream"
  out "Build ubuntu-demo-opae image:"
  command "cd demo; ./build-image.sh ubuntu-demo-opae ; cd ../" 15
  out "Import image from docker to CRI-O:"
  command "docker save ubuntu-demo-opae:devel | sudo podman load"
  out "Program bitstream that is not wanted by the workload:"
  command "sudo /opt/intel/fpga-sw/opae/fpgaconf-wrapper -s0 /srv/intel.com/fpga/ce48969398f05f33946d560708be108a/d8424dc4a4a3c413f89e433683f9040b.gbs"
  out "Check if device is programmed with it:"
  command "cat /sys/class/fpga/intel-fpga-dev.0/intel-fpga-port.0/afu_id"
  out "Run workload:"
  command "kubectl create -f demo/test-fpga-region.yml"
  sleep 2
  out "Look at the test output"
  command "kubectl logs test-fpga-region"
  out "Check if orchestration reprogrammed device with required(NLB3) bitstream:"
  command "cat /sys/class/fpga/intel-fpga-dev.0/intel-fpga-port.0/afu_id"
}

screen6()
{
  clear
  out "Summary:" 15
  out "This screencast demonstrated 'Orchestration programmed' use case for FPGA:" 15
  out " - FPGA device was programmed by the kubernetes machinery" 15
  out " - desired bitstream resource was specified in the pod spec as fpga.intel.com/arria10-nlb3" 15
  out " - the machinery mapped arria10-nlb3 into the pair of region id/AFU id using admission controller webhook" 15
  out " - programming was done by OPAE tools installed by the init container into /opt/intel/fpga-sw" 15
  out
  out "More detailed information about Intel Device Plugins can be found at https://github.com/intel/intel-device-plugins-for-kubernetes" 15
}

if [ "$1" == 'play' ] ; then
  if [ -n "$2" ] ; then
    screen$2
  else
    for n in $(seq 6) ; do screen$n ; sleep 3; done
  fi
elif [ "$1" == 'cleanup' ] ; then
  cleanup
elif [ "$1" == 'record' ] ; then
  record
else
   echo 'Usage: screencast.sh [--help|help|-h] | [play [<screen number>]] | [cleanup] | [record]'
fi
