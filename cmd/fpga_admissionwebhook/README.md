# Intel FPGA admission controller for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Dependencies](#dependencies)
* [Installation](#installation)
    * [Pre-requisites](#pre-requisites)
    * [Deployment](#deployment)
* [Mappings](#mappings)
* [Next steps](#next-steps)

## Introduction

The FPGA admission controller is one of the components used to add support for Intel FPGA
devices to Kubernetes.

> **NOTE:** Installation of the FPGA admission controller can be skipped if the
> [FPGA device plugin](../fpga_plugin/README.md) is operated with the Intel Device Plugins Operator
> since it integrates the controller's functionality.

The FPGA admission controller webhook is responsible for performing mapping from user-friendly
function IDs to the Interface ID and Bitstream ID that are required for FPGA programming by
the [FPGA CRI-O hook](../fpga_crihook/README.md).

Mappings are stored in namespaced custom resource definition (CRD) objects, therefore the admission
controller also performs access control, determining which bitstream can be used for which namespace.
More details can be found in the [Mappings](#mappings) section.

The admission controller also keeps the user from bypassing namespaced mapping restrictions,
by denying admission of any pods that are trying to use internal knowledge of InterfaceID or
Bitstream ID environment variables used by the prestart hook.

## Dependencies

This component is one of a set of components that work together. You may also want to
install the following:

-   [FPGA device plugin](../fpga_plugin/README.md)
-   [FPGA prestart CRI-O hook](../fpga_crihook/README.md)

All components have the same basic dependencies as the
[generic plugin framework dependencies](../../README.md#about)

## Installation

The following sections detail how to obtain, build and deploy the admission
controller webhook plugin.

### Pre-requisites

The webhook depends on having [cert-manager](https://cert-manager.io/)
installed:

```bash
$ kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml
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

To deploy the webhook, run

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/fpga_admissionwebhook/default?ref=master
namespace/intelfpgawebhook-system created
customresourcedefinition.apiextensions.k8s.io/acceleratorfunctions.fpga.intel.com created
customresourcedefinition.apiextensions.k8s.io/fpgaregions.fpga.intel.com created
mutatingwebhookconfiguration.admissionregistration.k8s.io/intelfpgawebhook-mutating-webhook-configuration created
clusterrole.rbac.authorization.k8s.io/intelfpgawebhook-manager-role created
clusterrolebinding.rbac.authorization.k8s.io/intelfpgawebhook-manager-rolebinding created
service/intelfpgawebhook-webhook-service created
deployment.apps/intelfpgawebhook-webhook created
certificate.cert-manager.io/intelfpgawebhook-serving-cert created
issuer.cert-manager.io/intelfpgawebhook-selfsigned-issuer created
```
Now you can deploy your mappings.

## Mappings

Mappings is a an essential part of the setup that gives a flexible instrument to a cluster
administrator to manage FPGA bitstreams and to control access to them. Being a set of
custom resource definitions they are used to configure the way FPGA resource requests get
translated into actual resources provided by the cluster.

For the following mapping

```yaml
apiVersion: fpga.intel.com/v2
kind: AcceleratorFunction
metadata:
  name: arria10.dcp1.2-nlb0-preprogrammed
spec:
  afuId: d8424dc4a4a3c413f89e433683f9040b
  interfaceId: 69528db6eb31577a8c3668f9faa081f6
  mode: af
```

requested FPGA resources are translated to AF resources. For example,
`fpga.intel.com/arria10.dcp1.2-nlb0-preprogrammed` is translated to
`fpga.intel.com/af-695.d84.aVKNtusxV3qMNmj5-qCB9thCTcSko8QT-J5DNoP5BAs` where the `af-`
prefix indicates the plugin's mode (`af`), `695` is the first three characters of
the region interface ID, `d84` is the first three characters of the accelerator function ID
and the last part `aVKNtusxV3qMNmj5-qCB9thCTcSko8QT-J5DNoP5BAs` is a base64-encoded concatenation
of the full region interface ID and accelerator function ID.
The format of resource names (e.g. `arria10.dcp1.2-nlb0-preprogrammed`) can be any and is up
to a cluster administrator.

The same mapping, but with its mode field set to `region`, would translate
`fpga.intel.com/arria10.dcp1.2-nlb0-preprogrammed` to `fpga.intel.com/region-69528db6eb31577a8c3668f9faa081f6`,
and the corresponding AF IDs are set in environment variables for the container.
Though in this case the cluster administrator would probably want to rename
the mapping `arria10.dcp1.2-nlb0-preprogrammed` to something like `arria10.dcp1.2-nlb0-orchestrated`
to reflect its mode. The [FPGA CRI-O hook](../fpga_crihook/README.md) then loads the requested
bitstream to a region before the container is started.

Mappings of resource names are configured with objects of `AcceleratorFunction` and
`FpgaRegion` custom resource definitions found respectively in
[`./deployment/fpga_admissionwebhook/crd/bases/fpga.intel.com_af.yaml`](/deployments/fpga_admissionwebhook/crd/bases/fpga.intel.com_acceleratorfunctions.yaml)
and [`./deployment/fpga_admissionwebhook/crd/bases/fpga.intel.com_region.yaml`](/deployments/fpga_admissionwebhook/crd/bases/fpga.intel.com_fpgaregions.yaml).

Mappings between 'names' and 'ID's are controlled by the admission controller
mappings collection file found in
[`./deployments/fpga_admissionwebhook/mappings-collection.yaml`](/deployments/fpga_admissionwebhook/mappings-collection.yaml).
This mappings file can be deployed with

```bash
$ kubectl apply -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/master/deployments/fpga_admissionwebhook/mappings-collection.yaml
```

Note that the mappings are scoped to the namespaces they were created in
and they are applicable to pods created in the corresponding namespaces.

## Next steps

Continue with [FPGA prestart CRI-O hook](../fpga_crihook/README.md).