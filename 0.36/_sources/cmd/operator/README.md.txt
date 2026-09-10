# Intel Device Plugins Operator

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
* [Upgrade](#upgrade)
* [Limiting Supported Devices](#limiting-supported-devices)
* [Known issues](#known-issues)

## Introduction

Intel Device Plugins Operator is a Kubernetes custom controller whose goal is to serve the
installation and lifecycle management of Intel device plugins for Kubernetes.
It provides a single point of control for GPU, QAT, SGX, FPGA, DSA and DLB devices to a cluster
administrators.

## Installation

The default operator deployment depends on NFD and cert-manager. Those components have to be installed to the cluster before the operator can be deployed.

> **Note**: Operator can also be installed via Helm charts. See [INSTALL.md](../../INSTALL.md) for details.

### NFD

Install NFD (if it's not already installed) and node labelling rules (requires NFD v0.13+):

```
# deploy NFD
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd?ref=<RELEASE_VERSION>'
# deploy NodeFeatureRules
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>'
```
Make sure both NFD master and worker pods are running:

```
$ kubectl get pods -n node-feature-discovery
NAME                          READY   STATUS    RESTARTS   AGE
nfd-master-599c58dffc-9wql4   1/1     Running   0          25h
nfd-worker-qqq4h              1/1     Running   0          25h
```

Note that labelling is not performed immediately. Give NFD 1 minute to pick up the rules and label nodes.

As a result all found devices should have correspondent labels, e.g. for Intel DLB devices the label is
`intel.feature.node.kubernetes.io/dlb`:
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

### Cert-Manager

> **Note**: The default deployment for the Intel Device Plugin operator uses self-signed certificates. For a production cluster, the certificate issuer should be properly set and not use a self-signed method.

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

### Device Plugin Operator

Finally deploy the operator itself:

```
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/default?ref=<RELEASE_VERSION>'
```

Now you can deploy the device plugins by creating corresponding custom resources.
The samples for them are available [here](/deployments/operator/samples/).

### Device Plugin Custom Resource

Deploy your device plugin by applying its custom resource, e.g.
`GpuDevicePlugin` with

```bash
$ kubectl apply -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/deployments/operator/samples/deviceplugin_v1_gpudeviceplugin.yaml
```

Observe it is up and running:

```bash
$ kubectl get GpuDevicePlugin
NAME                     DESIRED   READY   NODE SELECTOR   AGE
gpudeviceplugin-sample   1         1                       5s
```

**NOTE:** Intel Device Plugin Operator supports multiple custom resources per Kind (QAT, DSA, etc.). With multiple custom resources and different `nodeSelectors`, it is possible to customize device plugin configuration per node or per group of nodes. See also [known issues](#multiple-custom-resources).

## Upgrade

The upgrade of the deployed plugins can be done by simply installing a new release of the operator.

The operator auto-upgrades operator-managed plugins (CR images and thus corresponding deployed daemonsets) to the current release of the operator.

From `0.28.0` release, each version of the operator can have a set of images in `deployments/operator/manager/manager.yaml` as env variables.

When env variables are set for specific plugins (and their initcontainers), plugins are upgraded to the images set as env variables and all user input is ignored.

The name of env variables is capitalized image with '_SHA' ending (e.g. in case of the image for `intel-sgx-plugin`, the env variable is `INTEL_SGX_PLUGIN_SHA`).

The value of env variables is the full path of the image (e.g. `docker.io/intel/intel-sgx-plugin@sha256:<digest>`).

## Limiting Supported Devices

In order to limit the deployment to a specific device type,
use one of kustomizations under `deployments/operator/device`.

For example, to limit the deployment to FPGA, use:

```bash
$ kubectl apply -k deployments/operator/device/fpga
```

Operator also supports deployments with multiple selected device types.
In this case, create a new kustomization with the necessary resources
that passes the desired device types to the operator using `--device`
command line argument multiple times.

## Known issues

### Multiple Custom Resources

With multiple custom resources, `nodeSelector` has to be carefully set to avoid device plugin DaemonSet getting deployed multiple times on the same node, as operator does not check or prevent this. Multiple plugins managing same resource on a node can cause invalid behavior and/or duplicate device resources on node.

### Cluster behind a proxy

If your cluster operates behind a corporate proxy make sure that the API
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

### Leader election enabled

When the operator is run with leader election enabled, that is with the option
`--leader-elect`, make sure the cluster is not overloaded with excessive
number of pods. Otherwise a heart beat used by the leader election code may trigger
a timeout and crash. We are going to use different clients for the controller and
leader election code to alleviate the issue. See more details in
https://github.com/intel/intel-device-plugins-for-kubernetes/issues/476.

In case the deployment is limited to specific device type(s),
the CRDs for other device types are still created, but no controllers
for them are registered.
