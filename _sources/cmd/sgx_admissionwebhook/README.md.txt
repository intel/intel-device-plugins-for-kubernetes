# Intel SGX admission controller for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Pre-requisites](#pre-requisites)
    * [Deployment](#deployment)

## Introduction

The SGX admission controller is one of the components used to add support for Intel SGX
functionality to Kubernetes.

> **NOTE:** Installation of the SGX admission controller can be skipped if the
> [SGX device plugin](../sgx_plugin/README.md) is operated with the Intel Device Plugins Operator
> since it integrates the controller's functionality.

The SGX admission webhook is responsible for performing Pod mutations based on the `sgx.intel.com/quote-provider`
pod annotation set by the user. The purpose of the webhook is to hide the details of setting the necessary
device resources and volume mounts for using SGX remote attestation in the cluster. Furthermore,
the SGX admission webhook is responsible for writing a pod/sandbox `sgx.intel.com/epc` annotation that is used by
Kata Containers to dynamically adjust its virtualized SGX encrypted page cache (EPC) bank(s) size.

## Installation

The following sections detail how to obtain, build and deploy the admission
controller webhook plugin.

### Pre-requisites

The simplest webhook deployment depends on having [cert-manager](https://cert-manager.io/)
installed:

```bash
$ kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

Also if your cluster operates behind a corporate proxy make sure that the API
server is configured not to send requests to cluster services through the
proxy. You can check that with the following command:

```bash
$ kubectl describe pod kube-apiserver --namespace kube-system | grep -i no_proxy | grep "\.svc"
```

In case there's no output and your cluster was deployed with `kubeadm` open
`/etc/kubernetes/manifests/kube-apiserver.yaml` at the control plane nodes and
append `.svc` and `.svc.cluster.local` to the `no_proxy` environment variable:

```yaml
apiVersion: v1
kind: Pod
metadata:
  ...
spec:
  containers:
  - command:
    - kube-apiserver
    - --advertise-address=10.237.71.99
    ...
    env:
    - name: http_proxy
      value: http://proxy.host:8080
    - name: https_proxy
      value: http://proxy.host:8433
    - name: no_proxy
      value: 127.0.0.1,localhost,.example.com,10.0.0.0/8,.svc,.svc.cluster.local
    ...
```

**Note:** To build clusters using `kubeadm` with the right `no_proxy` settings from the very beginning,
set the cluster service names to `$no_proxy` before `kubeadm init`:

```
$ export no_proxy=$no_proxy,.svc,.svc.cluster.local
```

### Deployment

To deploy the webhook with cert-manager, run

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_admissionwebhook/overlays/default-with-certmanager?ref=main
```
