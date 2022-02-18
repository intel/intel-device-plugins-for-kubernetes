# Intel Device Plugins Operator

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
* [Upgrade](#upgrade)
* [Known issues](#known-issues)

## Introduction

Intel Device Plugins Operator is a Kubernetes custom controller whose goal is to serve the
installation and lifecycle management of Intel device plugins for Kubernetes.
It provides a single point of control for GPU, QAT, SGX, FPGA, DSA and DLB devices to a cluster
administrators.

## Installation

Install NFD (if it's not already installed) and node labelling rules (requires NFD v0.10+):

```
# either with default NFD installation
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd?ref=<RELEASE_VERSION>
# or when setting up with SGX
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/sgx?ref=<RELEASE_VERSION>
# and finally, NodeFeatureRules
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>
```
Make sure both NFD master and worker pods are running:

$ kubectl get pods -n node-feature-discovery
NAME                          READY   STATUS    RESTARTS   AGE
nfd-master-599c58dffc-9wql4   1/1     Running   0          25h
nfd-worker-qqq4h              1/1     Running   0          25h

Note that labelling is not performed immediately. Give NFD 1 minute to pick up the rules and label nodes.

As a result all found devices should have correspondent labels, e.g. for Intel DLB devices the label is
intel.feature.node.kubernetes.io/dlb:
```
$ kubectl get no -o json | jq .items[].metadata.labels |grep intel.feature.node.kubernetes.io/dlb
  "intel.feature.node.kubernetes.io/dlb": "true",
```

Full list of labels can be found in the deployments/operator/samples directory:
```
$ grep -r feature.node.kubernetes.io/ deployments/operator/samples/
deployments/operator/samples/deviceplugin_v1_dlbdeviceplugin.yaml:    intel.feature.node.kubernetes.io/dlb: 'true'
deployments/operator/samples/deviceplugin_v1_qatdeviceplugin.yaml:    intel.feature.node.kubernetes.io/qat: 'true'
deployments/operator/samples/deviceplugin_v1_sgxdeviceplugin.yaml:    intel.feature.node.kubernetes.io/sgx: 'true'
deployments/operator/samples/deviceplugin_v1_gpudeviceplugin.yaml:    intel.feature.node.kubernetes.io/gpu: "true"
deployments/operator/samples/deviceplugin_v1_fpgadeviceplugin.yaml:    intel.feature.node.kubernetes.io/fpga-arria10: 'true'
deployments/operator/samples/deviceplugin_v1_dsadeviceplugin.yaml:    intel.feature.node.kubernetes.io/dsa: 'true'
```

The default operator deployment depends on [cert-manager](https://cert-manager.io/) running in the cluster.
See installation instructions [here](https://cert-manager.io/docs/installation/kubectl/).

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

## Upgrade

The upgrade of the deployed plugins can be done by simply installing a new release of the operator.

The operator auto-upgrades operator-managed plugins (CR images and thus corresponding deployed daemonsets) to the current release of the operator.

The [registry-url]/[namespace]/[image] are kept intact on the upgrade.

No upgrade is done for:

- Non-operator managed deployments
- Operator deployments without numeric tags

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
