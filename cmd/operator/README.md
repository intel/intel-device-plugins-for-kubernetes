# Intel One Operator for Device Plugins

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)

## Introduction

This One Operator is a Kubernetes custom controller whose goal is to serve the
installation and lifecycle management of Intel device plugins for Kubernetes.
It provides a single point of control for GPU, QAT and FPGA devices to a cluster
administrators.

## Installation

The operator depends on [cert-manager](https://cert-manager.io/) running in the cluster.
To install it run:

```
$ kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.0/cert-manager.yaml
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
append `.svc` to the `no_proxy` environment variable:

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
      value: 127.0.0.1,localhost,.example.com,10.0.0.0/8,.svc
    ...
```

Finally deploy the operator itself:

```
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/default?ref=master
```

Now you can deploy the device plugins by creating corresponding custom resources.
The samples for them are available [here](/deployments/operator/samples/).
