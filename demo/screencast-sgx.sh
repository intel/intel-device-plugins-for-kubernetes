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
  out 'delete node-feature-discovery deployment:' 20
  command 'kubectl delete -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_nfd?ref=master || true' 20
  out 'delete SGX Device Plugin deployment:' 20
  command 'kubectl delete sgxdeviceplugin sgxdeviceplugin-sample -n sgx-ecdsa-quote || true' 20
  out 'delete Intel Device Plugin Operator deployment:' 20
  command 'kubectl delete -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/default?ref=master || true' 20
  out "delete the demo namespace"
  command "kubectl delete ns sgx-ecdsa-quote"
}

record()
{
  clear
  out 'Record this screencast'
  command 'asciinema rec -t "Intel SGX Device Plugin for Kubernetes - Intel(R) SGX DCAP ECDSA Quote Generation Demo"  Intel-SGX-Device-Plugin-for-Kubernetes-SGX-DCAP-ECDSA-Quote-Generation-Demo.cast -c "./screencast-sgx.sh play"'
}

screen1()
{
  clear
  out "This video demonstrates the Intel(R) Software Guard Extensions ECDSA Quote Generation in Kubernetes*"
  out "The key building blocks are:"
  out "* Intel(R) Software Guard Extensions (SGX) Flexible Launch Control capable system (registered)"
  out "* Intel(R) SGX driver (RFC v41) for the host kernel"
  out "* Intel(R) SGX PCKID Certificate Caching Service configured"
  out "Let's get started!"
}

screen2()
{
  clear
  out "1. Check the Kubernetes cluster is in good shape"
  command "kubectl get nodes"
  command "kubectl get pods --all-namespaces"
  out "Create the demo namespace"
  command "kubectl create ns sgx-ecdsa-quote"
  out "Pull :devel images and tag them as :0.19.0 (temporary until the release is out)"
  command "sudo ctr -n k8s.io i pull docker.io/intel/intel-sgx-plugin:devel"
  command "sudo ctr -n k8s.io i pull docker.io/intel/intel-sgx-initcontainer:devel"
  command "sudo ctr -n k8s.io i tag docker.io/intel/intel-sgx-plugin:devel docker.io/intel/intel-sgx-plugin:0.19.0"
  command "sudo ctr -n k8s.io i tag docker.io/intel/intel-sgx-initcontainer:devel docker.io/intel/intel-sgx-initcontainer:0.19.0"
}

screen3()
{
  clear
  out "2. Deploy node-feature-discovery for Kubernetes"
  out "It's used to label SGX capable nodes and register SGX EPC as an extended resource"
  command "kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_nfd?ref=master"
  out "Check its pod is running"
  command "kubectl wait --for=condition=Ready pod/$(kubectl get --no-headers -l app=nfd-worker -o=jsonpath='{.items[0].metadata.name}' pods -n node-feature-discovery) -n node-feature-discovery"
}

screen4()
{
  clear
  out "3. Deploy Intel Device Plugin Operator"
  command "kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/default?ref=master"
  out "Create SgxDevicePlugin custom resource managed by the Operator"
  command "kubectl apply -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/master/deployments/operator/samples/deviceplugin_v1_sgxdeviceplugin.yaml -n sgx-ecdsa-quote"
  out "Check the SGX Device Plugin is running"
  command "kubectl get pods -n sgx-ecdsa-quote"
}

screen5()
{
  clear
  out "4. Verify node resources"
  command "kubectl get nodes -o json | jq .items[].status.allocatable | grep -i sgx"
  command "kubectl get nodes -o json | jq .items[].metadata.labels | grep SGX"
  out "Both node labels and resources for SGX are in place"
}

screen6()
{
  clear
  out "5. Run Intel(R) SGX DCAP ECDSA Quote Generation (out-of-proc)"
  out "Make the pre-built images available (from docker save)"
  command "sudo ctr -n k8s.io i import sgx-aesmd.tar"
  command "sudo ctr -n k8s.io i import sgx-demo.tar"
  out "Deploy Intel(R) AESMD"
  command "kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_aesmd?ref=master -n sgx-ecdsa-quote"
  out "Deploy Intel(R) SGX DCAP ECDSA Quote Generation"
  command "kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_enclave_apps/overlays/sgx_ecdsa_aesmd_quote?ref=master -n sgx-ecdsa-quote"
  command "kubectl logs $(kubectl get --no-headers -l job-name=ecdsa-quote-intelsgx-demo-job -o=jsonpath='{.items[0].metadata.name}' pods -n sgx-ecdsa-quote) -n sgx-ecdsa-quote"
  out "Intel(R) SGX DCAP QuoteGenerationSample successfully requested a quote from Intel(R) AESMD"
  out "Delete the deployment"
  command "kubectl delete -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_aesmd?ref=master -n sgx-ecdsa-quote"
  command "kubectl delete -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_enclave_apps/overlays/sgx_ecdsa_aesmd_quote?ref=master -n sgx-ecdsa-quote"
}

screen7()
{
  clear
  out "6. Run Intel(R) SGX DCAP ECDSA Quote Generation (in-proc)"
  out "Deploy Intel(R) SGX DCAP ECDSA Quote Generation"
  command "kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_enclave_apps/overlays/sgx_ecdsa_inproc_quote?ref=master -n sgx-ecdsa-quote"
  command "kubectl logs $(kubectl get --no-headers -l job-name=inproc-ecdsa-quote-intelsgx-demo-job -o=jsonpath='{.items[0].metadata.name}' pods -n sgx-ecdsa-quote) -n sgx-ecdsa-quote"
  out "Intel(R) SGX DCAP QuoteGenerationSample successfully generated a quote using DCAP Quote Provider Library"
  out "Delete the deployment"
  command "kubectl delete -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_enclave_apps/overlays/sgx_ecdsa_inproc_quote?ref=master -n sgx-ecdsa-quote"
}

screen8()
{
  clear
  out "This video demonstrated the Intel(R) Software Guard Extensions in Kubernetes*"
  out "The following topics were covered:"
  out "* SGX Kubernetes* Device Plugin deployment with an Operator"
  out "* Intel(R) SGX node resource and feature label registration to Kubernetes*"
  out "* Intel(R) SGX DCAP ECDSA Quote Generation (out-of-proc and in-proc)"
}

if [ "$1" == 'play' ] ; then
  if [ -n "$2" ] ; then
    screen$2
  else
    for n in $(seq 8) ; do screen$n ; sleep 3; done
  fi
elif [ "$1" == 'cleanup' ] ; then
  cleanup
elif [ "$1" == 'record' ] ; then
  record
else
   echo 'Usage: screencast-sgx.sh [--help|help|-h] | [play [<screen number>]] | [cleanup] | [record]'
fi
