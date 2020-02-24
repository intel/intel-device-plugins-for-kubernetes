# Intel FPGA admission controller for Kubernetes

# Table of Contents

* [Introduction](#introduction)
* [Dependencies](#dependencies)
* [Installation](#installation)
    * [Getting the source code](#getting-the-source-code)
    * [Deploying via the script](#deploying-via-the-script)
        * [Pre-requisites](#pre-requisites)
        * [Build the webhook image](#build-the-webhook-image)
        * [Deploying via the script](#deploying-via-the-script)
* [Mappings](#mappings)
* [Next steps](#next-steps)

# Introduction

The FPGA admission controller is one of the components used to add support for Intel FPGA
devices to Kubernetes.

The FPGA admission controller webhook is responsible for performing mapping from user-friendly
function IDs to the Interface ID and Bitstream ID that are required for FPGA programming by
the [FPGA CRI-O hook](../fpga_crihook/README.md).

Mappings are stored in namespaced custom resource definition (CRD) objects, therefore the admission
controller also performs access control, determining which bitstream can be used for which namespace.
More details can be found in the [Mappings](#mappings) section.

The admission controller also keeps the user from bypassing namespaced mapping restrictions,
by denying admission of any pods that are trying to use internal knowledge of InterfaceID or
Bitstream ID environment variables used by the prestart hook.

The admission controller can operate in two separate modes - preprogrammed or orchestration programmed.
The mode must be chosen to match that of the [FPGA plugin](../fpga_plugin/README.md) configuraton, as
shown in the following table:

| FPGA plugin mode | matching admission controller mode |
|:---------------- |:---------------------------------- |
| region           | orchestrated                       |
| af               | preprogrammed                      |


# Dependencies

This component is one of a set of components that work together. You may also want to
install the following:

-   [FPGA device plugin](../fpga_plugin/README.md)
-   [FPGA prestart CRI-O hook](../fpga_crihook/README.md)

All components have the same basic dependencies as the
[generic plugin framework dependencies](../../README.md#about)

# Installation

The following sections detail how to obtain, build and deploy the admission
controller webhook plugin.

## Getting the source code

```bash
$ mkdir -p $(go env GOPATH)/src/github.com/intel
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
```

## Deploying via the script

Deploying the webhook admission controller consists of a number of components,
and actions, including:

- Certificates (secrets)
- CRDs
- Deployment
- Service
- Registration

A script is provided to make the whole process easier.

### Pre-requisites

The script has some pre-requisite tools that must be installed on your system:
- [`jq`](https://github.com/stedolan/jq)
- [`cfssl`](https://github.com/cloudflare/cfssl)

### Build the webhook image

Before the webhook can be deployed, its container image needs to be built:

```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make intel-fpga-admissionwebhook
...
Successfully tagged intel/intel-fpga-admissionwebhook:devel
```

### Deploying via the script

To deploy the webhook, run the [`scripts/webhook-deploy.sh`](../../scripts/webhook-deploy.sh)
 script:

```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ ./scripts/webhook-deploy.sh
Create secret including signed key/cert pair for the webhook
...
Create FPGA CRDs
customresourcedefinition.apiextensions.k8s.io/acceleratorfunctions.fpga.intel.com created
customresourcedefinition.apiextensions.k8s.io/fpgaregions.fpga.intel.com created
...
clusterrole.rbac.authorization.k8s.io/fpga-reader created
clusterrolebinding.rbac.authorization.k8s.io/default-fpga-reader created
...
Create webhook deployment
deployment.extensions/intel-fpga-webhook-deployment created
Create webhook service
service/intel-fpga-webhook-svc created
Register webhook
mutatingwebhookconfiguration.admissionregistration.k8s.io/fpga-mutator-webhook-cfg created
```

By default, the script deploys the webhook in a preprogrammed mode.

Use the option `--mode` script option to deploy the webhook in orchestrated mode:

```bash
$ ./scripts/webhook-deploy.sh --mode orchestrated
```

The script needs the CA bundle used for signing certificate requests in your cluster.
By default, the script fetches the bundle stored in the configmap
`extension-apiserver-authentication`. However, your cluster may use a different signing
certificate that is passed in the option `--cluster-signing-cert-file` to `kube-controller-manager`.
In this case, you must point the script to the actual signing certificate as follows:

```bash
$ ./scripts/webhook-deploy.sh --ca-bundle-path /var/run/kubernetes/server-ca.crt
```

# Mappings

Requested FPGA resources are translated to AF resources. For example,
`fpga.intel.com/arria10.dcp1.1-nlb0` is translated to `fpga.intel.com/af-d8424dc4a4a3c413f89e433683f9040b`.

In orchestrated mode, `fpga.intel.com/arria10.dcp1.1-nlb0` gets translated to
`fpga.intel.com/region-9926ab6d6c925a68aabca7d84c545738`, and, the corresponding AF IDs are set in
environment variables for the container. The [FPGA CRI-O hook](../fpga_crihook/README.md)
then loads the requested bitstream to a region before the container is started.

Mappings of resource names are configured with objects of `AcceleratorFunction` and
`FpgaRegion` custom resource definitions found respectively in
[`./deployment/fpga_admissionwebhook/af-crd.yaml`](../../deployment/fpga_admissionwebhook/af-crd.yaml)
and [`./deployment/fpga_admissionwebhook/region-crd.yaml`](../../deployment/fpga_admissionwebhook/region-crd.yaml.)

Mappings between 'names' and 'ID's are controlled by the admission controller
mappings collection file found in
[`./deployments/fpga_admissionwebhook/mappings-collection.yaml`](../../deployments/fpga_admissionwebhook/mappings-collection.yaml).
This mappings file is deployed alongside the admission controller as part of the
[webhook deploy script](../../scripts/webhook-deploy.sh).

Note that the mappings are scoped to the namespaces they were created in
and they are applicable to pods created in the corresponding namespaces.

# Next steps

Continue with [FPGA prestart CRI-O hook](../fpga_crihook/README.md).
