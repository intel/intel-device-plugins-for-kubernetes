# Intel Device Plugins Operator

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
* [Known issues](#known-issues)

## Introduction

Intel Device Plugins Operator is a Kubernetes custom controller whose goal is to serve the
installation and lifecycle management of Intel device plugins for Kubernetes.
It provides a single point of control for GPU, QAT, SGX and FPGA devices to a cluster
administrators.

## Installation

The operator depends on [cert-manager](https://cert-manager.io/) running in the cluster.
To install it run:

```
$ kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
```

Make sure all the pods in the `cert-manager` namespace are up and running:

```
$ kubectl get pods -n cert-manager
NAME                                      READY   STATUS    RESTARTS   AGE
cert-manager-7747db9d88-bd2nl             1/1     Running   0          21d
cert-manager-cainjector-87c85c6ff-59sb5   1/1     Running   0          21d
cert-manager-webhook-64dc9fff44-29cfc     1/1     Running   0          21d
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

Finally deploy the operator itself:

```
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/default?ref=<RELEASE_VERSION>
```

Now you can deploy the device plugins by creating corresponding custom resources.
The samples for them are available [here](/deployments/operator/samples/).

## Usage

Deploy your device plugin by applying its custom resource, e.g.
`GpuDevicePlugin` with

```bash
$ kubectl apply -f https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/samples/deviceplugin_v1_gpudeviceplugin.yaml
```

Observe it is up and running:

```bash
$ kubectl get GpuDevicePlugin
NAME                     DESIRED   READY   NODE SELECTOR   AGE
gpudeviceplugin-sample   1         1                       5s
```

In order to limit the deployment to a specific device type,
use one of kustomizations under deployments/operator/device.

For example, to limit the deployment to FPGA, use:

```bash
$ kubectl apply -k deployments/operator/device/fpga
```

Operator also supports deployments with multiple selected device types.
In this case, create a new kustomization with the necessary resources
that passes the desired device types to the operator using `--device`
command line argument multiple times.

## Known issues

When the operator is run with leader election enabled, that is with the option
`--enable-leader-election`, make sure the cluster is not overloaded with excessive
number of pods. Otherwise a heart beat used by the leader election code may trigger
a timeout and crash. We are going to use different clients for the controller and
leader election code to alleviate the issue. See more details in
https://github.com/intel/intel-device-plugins-for-kubernetes/issues/476.

In case the deployment is limited to specific device type(s),
the CRDs for other device types are still created, but no controllers
for them are registered.
