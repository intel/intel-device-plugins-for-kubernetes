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
  command 'asciinema rec -t "Intel QAT Device Plugin for Kubernetes - Test Demo"  Intel-QAT-Device-Plugin-for-Kubernetes-Test-Demo.cast -c "sh ./qat-dp-demo.sh play"'
}
 screen1()
{
 # cleanup
  clear
  out "This video demonstrates the Intel(R) QuickAssist Technology device Plugin for Kubernetes"
  out "Prerequisites:"
  out "* Intel(R) QuickAssist Technology (Intel(R) QAT) DH895xcc Chipset with SR-IOV enabled"
  out "* Kubernetes v1.12 cluster"
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
  out "7. View pod specification file for pod requesting QAT Virtual Functions:"
  command "cat crypto-perf-dpdk-pod-requesting-qat.yaml"
  sleep 5 
  out "8. Create Pod requesting QAT Virtual Functions:"
  command "kubectl create -f crypto-perf-dpdk-pod-requesting-qat.yaml"
  sleep 2
  command "kubectl get pods"
  sleep 2
}
 screen6()
{
  clear
  out "9. Get a shell to the running container and run a DPDK application using QAT device"
  out "* \"export\": Lists environment variables - note QAT0, QAT1, QAT2... etc"
  out "* \"./dpdk-test-crypto-perf -l 6-7 -w \$QAT0 -- --ptest throughput --devtype crypto_qat --optype cipher-only --cipher-algo aes-cbc --cipher-op encrypt --cipher-key-sz 16 --total-ops 10000000 --burst-sz 32 --buffer-sz 64\" : Manually executes the dpdk-test-crypto-perf application to review the logs"
   command "kubectl exec -it dpdkqatuio bash"
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
   echo 'Usage: screencast.sh [--help|help|-h] | [play [<screen number>]] | [cleanup] | [record]'
fi
