#!/bin/bash -e

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
  out 'delete test pods:' 20
  command 'kubectl delete pod openssl-qat-engine -n openssl-qat-engine || true' 20
  out 'delete plugin configmap:' 20
  command 'kubectl delete configmap intel-qat-plugin-config -n openssl-qat-engine || true' 20
  out 'delete plugin daemonset:' 20
  command 'kubectl delete daemonset intel-qat-plugin -n openssl-qat-engine || true' 20
  out 'delete demo namespace:' 20
  command 'kubectl delete namespace openssl-qat-engine || true' 20
  out 'unload QAT kernel modules:' 20
  command 'sudo rmmod qat_c62xvf qat_c62x intel_qat'
}

record()
{
  clear
  out 'Record this screencast'
  command "asciinema rec -t 'Intel QAT Device Plugin for Kubernetes - Intel(R) QAT OpenSSL Engine Demo'  Intel-QAT-Device-Plugin-for-Kubernetes-OpenSSL-QAT-Engine-Demo.cast -c 'sh $0 play'"
}

screen1()
{
  clear
  out "This video demonstrates the Intel(R) QuickAssist Technology device Plugin for Kubernetes*"
  out "The key building blocks are:"
  out "* Intel(R) QuickAssist Technology (Intel(R) QAT) C62x Chipset with SR-IOV enabled"
  out "* A Kubernetes v1.12 cluster with RuntimeClass and Device Plugins enabled"
  out "* containerd v1.2 CRI and Kata Containers* runtime"
  out "* Intel(R) QAT driver modules for host and Kata Containers kernels"
  out "* OpenSSL* 1.1 and Intel(R) QAT OpenSSL Engine installed in a container image"
  out "Let's get started!"
}

screen2()
{
  clear
  out "1. Prepare the host drivers and virtual function (VF) devices"
  out "Check the chip is available (C62x PCI ID 0x37c8)"
  command "/sbin/lspci -d 8086:37c8"
  out "Load the host driver modules"
  command "sudo dmesg -C"
  command "cat load-modules.sh"
  command "./load-modules.sh && dmesg"
  command "/sbin/lsmod | grep qat"
  out "Enable C62x SR-IOV (C62x VF PCI ID 0x37c9)"
  command "echo 16|sudo tee /sys/bus/pci/devices/$(/sbin/lspci -d 8086:37c8 -m -D|head -1|awk '{print $1}')/sriov_numvfs"
  command "/sbin/lspci -d 8086:37c9"
}

screen3()
{
  clear
  out "2. Check the Kubernetes cluster is in good shape"
  command "kubectl get nodes"
  command "kubectl get pods --all-namespaces"
  command "kubectl get runtimeclass"
  command "sudo crictl ps"
  out "Create the demo namespace"
  command "kubectl create ns openssl-qat-engine"
}

screen4()
{
  clear
  out "3. Deploy the Intel(R) QAT device plugin for Kubernetes"
  out "Load the container image to containerd CRI"
  command "sudo ctr cri load intel-qat-plugin.tar"
  out "Create the plugin configmap"
  command "kubectl create -f qat_plugin_demo_configmap.yaml -n openssl-qat-engine"
  command "kubectl describe configmap intel-qat-plugin-config -n openssl-qat-engine"
  out "Create the plugin daemonset"
  command "kubectl create -f qat_plugin.yaml -n openssl-qat-engine"
  out "Check its pod is runnning"
  command "kubectl get pods -n openssl-qat-engine | grep intel-qat-plugin"
}

screen5()
{
  clear
  out "4. Deploy Intel(R) QAT Accelerated OpenSSL workload"
  out "Load container image to containerd"
  command "sudo ctr cri load openssl-qat-engine.tar"
  out "Review the Pod Spec"
  command "cat openssl-qat-engine-pod.yaml"
  out "Deploy the workload"
  command "kubectl create -f openssl-qat-engine-pod.yaml -n openssl-qat-engine"
  out "Check the workload is running"
  command "kubectl get pods -n openssl-qat-engine"
}

screen6()
{
  clear
  out "5. Testing!"
  command "kubectl exec -it openssl-qat-engine -n openssl-qat-engine -- adf_ctl restart"
  command "kubectl exec -it openssl-qat-engine -n openssl-qat-engine -- adf_ctl status"
  command "kubectl exec -it openssl-qat-engine -n openssl-qat-engine -- openssl engine -c -t qat"
}

screen7()
{
  clear
  out "Summary:" 15
  out "This demonstrated Intel(R) QAT accelerated SSL/TLS for workloads using OpenSSL" 15
  out " - Host Intel(R) QAT VF resources controlled by Intel QAT Device plugin for Kubernetes" 15
  out " - Kata Containers enabled workloads get to see unique Intel(R) QAT devices" 15
  out " - Unmodified Intel(R) QAT user space stack and OpenSSL QAT Engine available for containers" 15
  out
  out "More detailed information about Intel Device Plugins can be found at https://github.com/intel/intel-device-plugins-for-kubernetes" 15
  DISCLAIMER=$(cat <<-EOD
Disclaimer:

Intel technologies’ features and benefits depend on system configuration and
may require enabled hardware, software or service activation. Learn more at
intel.com, or from the OEM or retailer.

No computer system can be absolutely secure.

Tests document performance of components on a particular test, in specific
systems. Differences in hardware, software, or configuration will affect actual
performance. Consult other sources of information to evaluate performance as
you consider your purchase.  For more complete information about performance
and benchmark results, visit http://www.intel.com/performance.

Demo platform configuration:

2x Intel(R) Xeon(R) CPU E5-2687W v4 @ 3.00GHz;
Total memory 251 GB; Intel(R) QAT C62x;

OpenSUSE* 15 (Kernel: 4.12.14-lp150.12.25-default); 1-node Kubernetes* 1.12;
Containerd v1.2; QAT 1.7 L.4.3.0-00033.

Intel(R) QAT Device Plugin built from main branch.

Intel and the Intel logo are trademarks of Intel Corporation in the U.S.
and/or other countries.

*Other names and brands may be claimed as the property of others.

(C) 2018 Intel Corporation.
EOD
)
  out "$DISCLAIMER" 1024
}

if [ "$1" == 'play' ] ; then
  if [ -n "$2" ] ; then
    screen$2
  else
    for n in $(seq 7) ; do screen$n ; sleep 3; done
  fi
elif [ "$1" == 'cleanup' ] ; then
  cleanup
elif [ "$1" == 'record' ] ; then
  record
else
   echo "Usage: $0 [--help|help|-h] | [play [<screen number>]] | [cleanup] | [record]"
fi
