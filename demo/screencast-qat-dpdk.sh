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
  kubectl delete daemonset intel-qat-plugin
  kubectl delete pod dpdkqatuio
  #out "cleanup done"
}
 record()
{
  clear
  out 'Record this screencast'
  command "asciinema rec -t 'Intel QAT Device Plugin for Kubernetes - Test Demo'  Intel-QAT-Device-Plugin-for-Kubernetes-Test-Demo.cast -c 'sh $0 play'"
}
 screen1()
{
 # cleanup
  clear
  out "This video demonstrates the Intel(R) QuickAssist Technology device Plugin for Kubernetes"
  out "Prerequisites:"
  out "* Intel(R) QuickAssist Technology (Intel(R) QAT) DH895xcc Chipset with SR-IOV enabled"
  out "* Kubernetes v1.14 cluster"
  out "* Data Plane Development Kit (DPDK) drivers configured"
  out "Let's get started!"
}
 screen2()
{
  clear
  out "1. Check if Kubernetes nodes are healthy:"
  command "kubectl get nodes"
  command "kubectl get pods"
  out "2. Check for allocatable resources:"
  command "kubectl get nodes -o json | jq .items[].status.allocatable"
  out "Here we see there are 0 allocatable QAT Virtual Functions"
  sleep 5
  out "3. List QAT Virtual Functions:"
  command "/root/dpdk/usertools/dpdk-devbind.py -s | grep \"QAT Virtual Function\""
  out "QAT Virtual Functions are bound to kernel drivers"
  sleep 3
}
 screen3()
{
  clear
  out "4. Deploy QAT device plugin as a DaemonSet:"
  command "kubectl create -f ../deployments/qat_plugin/qat_plugin_default_configmap.yaml"
  command "kubectl create -f ../deployments/qat_plugin/qat_plugin.yaml"
  command "kubectl get daemonset"
  command "kubectl get pods"
  sleep 3
}
 screen4()
{
  clear
  out "5. Check again for allocatable resources:"
  command "kubectl get nodes -o json | jq .items[].status.allocatable"
  out "Now we see there are 32 allocatable QAT Virtual Functions"
  sleep 5
  out "6. List QAT Virtual Functions:"
  command "/root/dpdk/usertools/dpdk-devbind.py -s | grep \"QAT Virtual Function\""
  out "Now QAT Virtual Functions are bound to DPDK driver"
  sleep 3
}
 screen5()
{
  clear
  out "7. View pod specification file for pod requesting QAT Virtual Functions (crypto):"
  command "kubectl apply --dry-run -o yaml -k ../deployments/qat_dpdk_app/crypto-perf"
  sleep 5 
  out "8. Create Pod requesting QAT Virtual Functions (crypto):"
  command "kubectl apply -k ../deployments/qat_dpdk_app/crypto-perf"
  sleep 2
  command "kubectl get pods"
  sleep 2
}
 screen6()
{
  clear
  out "9. View pod specification file for pod requesting QAT Virtual Functions (compress):"
  command "kubectl apply --dry-run -o yaml -k ../deployments/qat_dpdk_app/compress-perf"
  sleep 5 
  out "10. Create Pod requesting QAT Virtual Functions (compress):"
  command "kubectl apply -k ../deployments/qat_dpdk_app/compress-perf"
  sleep 2
  command "kubectl get pods"
  sleep 10
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
