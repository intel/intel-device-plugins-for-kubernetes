#!/bin/bash -e


# Check if DSA devices exist
if [ $(ls /dev/dsa/ |wc -l) -lt 2 ] ; then
    >&2 echo 'ERROR: This screencasts requires 2 DSA devices in /dev/dsa'
    exit 1
fi


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
  out 'Cleanup demo artifacts' 200
  command 'kubectl delete pod dsa-accel-config-demo || true' 200
  command 'kubectl delete -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/dsa_plugin || true' 200
}

record()
{
  clear
  out 'Record this screencast'
  command "asciinema rec -t 'Intel DSA Device Plugin for Kubernetes.'  Intel-DSA-Device-Plugin-for-Kubernetes-Demo.cast -c 'sh $0 play'" 300
}

screen1()
{
  clear
  out "This screencast demonstrates deployment of the Intel DSA Plugin for Kubernetes"
  out "Let's get started!"
  out 'Check if DSA devices are available on the node:'
  command 'accel-config list |grep "dev.*wq"'
  sleep 2
  command 'ls -la /dev/dsa/'
  out 'Check if Kubernetes node is in good shape:'
  command 'kubectl get nodes'
  command 'kubectl get pods -n kube-system'
  sleep 2
}

screen2()
{
  clear
  out 'Deploy DSA plugin'
  command 'kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/dsa_plugin' 100
  sleep 2
  out 'Check if the plugin pod is running:'
  command 'kubectl get pods | grep dsa'
  sleep 2
  out 'Check if DSA resources are allocatable:'
  command 'kubectl describe node |grep -A3 Allocatable'
  sleep 2
}

screen3()
{
  clear
  out 'Run test workload that uses DSA'
  command 'curl https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/demo/dsa-accel-config-demo-pod.yaml' 100
  command 'curl https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/demo/dsa-accel-config-demo/Dockerfile' 100
  command 'kubectl create -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/demo/dsa-accel-config-demo-pod.yaml' 100
  sleep 5
  out 'Look at the workload output'
  command 'kubectl logs -f dsa-accel-config-demo'
  sleep 2
}

screen4()
{
  clear
  out 'Summary:' 15
  out "This screencast demonstrated deployment and usage of the DSA device plugin" 15
  #out
  out 'More detailed information about Intel Device Plugins can be found at https://github.com/intel/intel-device-plugins-for-kubernetes' 15
}

if [ "$1" == 'play' ] ; then
  if [ -n "$2" ] ; then
    screen$2
  else
    for n in $(seq 4) ; do screen$n ; sleep 3; done
  fi
elif [ "$1" == 'cleanup' ] ; then
  cleanup
elif [ "$1" == 'record' ] ; then
  record
else
   echo "Usage: $0 [--help|help|-h] | [play [<screen number>]] | [cleanup] | [record]"
fi
