# Intel Software Guard Extensions (SGX) device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Modes and Configuration Options](#modes-and-configuration-options)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
    * [Pre-built Images](#pre-built-images)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Testing and Demos](#testing-and-demos)
    * [SGX ECDSA Remote Attestation](#sgx-ecdsa-remote-attestation)
        * [Remote Attestation Prerequisites](#remote-attestation-prerequisites)
        * [Build the images](#build-the-image)
        * [Deploy the pod](#deploy-the-pod)

## Introduction

The Intel SGX device plugin and related components allow workloads to use Intel SGX on
platforms with SGX Flexible Launch Control enabled, e.g.,:

- 3rd/4th Generation Intel® Xeon® Scalable Platforms
- Intel® Xeon® E3
- Intel® NUC Kit NUC7CJYH

The SGX solution comes in three parts:

- the [SGX Device plugin](/README.md#sgx-device-plugin)
- the [SGX Admission webhook](/README.md#sgx-admission-webhook)
- the [SGX EPC memory registration](/README.md#sgx-epc-memory-registration)

This README covers setting up all three components.

### Modes and Configuration options

The SGX plugin can take a number of command line arguments, summarised in the following table:

| Flag | Argument | Meaning |
|:---- |:-------- |:------- |
| -enclave-limit | int | the number of containers per worker node allowed to use `/dev/sgx_enclave` device node (default: `20`) |
| -provision-limit | int | the number of containers per worker node allowed to use `/dev/sgx_provision` device node (default: `20`) |

The plugin also accepts a number of other arguments related to logging. Please use the `-h` option to see
the complete list of logging related options.

## Installation

The following sections cover how to use the necessary Kubernetes SGX specific
components.

### Prerequisites

The component has the same basic dependencies as the
[generic plugin framework dependencies](../../README.md#about).

The SGX device plugin requires Linux Kernel SGX drivers to be available. These drivers
are available in Linux since 5.11. [The SGX DCAP out-of-tree driver](https://github.com/intel/SGXDataCenterAttestationPrimitives)
(v1.41 and later) is also known to work.

The hardware platform must support SGX Flexible Launch Control.

The SGX deployment depends on having [cert-manager](https://cert-manager.io/)
installed. See its installation instructions [here](https://cert-manager.io/docs/installation/kubectl/).

### Pre-built Images

[Pre-built images](https://hub.docker.com/u/intel/)
are available on Docker Hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on Docker Hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy Intel SGX components in your cluster is to follow the steps
below.

The deployment YAML files supplied with the components in this repository use the images with the `devel`
tag by default. If you do not build your own local images, your Kubernetes cluster may pull down
the devel images from Docker Hub by default.

Where `<RELEASE_VERSION>` needs to be substituted with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

### Installation Using the Operator

First, deploy `node-feature-discovery`:

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/?ref=<RELEASE_VERSION>'
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>'
```

**Note:** The [default configuration](/deployments/nfd/overlays/node-feature-rules/node-feature-rules.yaml) assumes that the in-tree driver is used and enabled (`CONFIG_X86_SGX=y`). If
the SGX DCAP out-of-tree driver is used, the `kernel.config` match expression must be removed.

Next, deploy the Intel Device plugin operator:

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/default?ref=<RELEASE_VERSION>'
```

**Note:** See the operator [deployment details](/cmd/operator/README.md) for its dependencies and for setting it up on systems behind proxies.

Finally, deploy the SGX device plugin with the operator

```bash
$ kubectl apply -f 'https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/<RELEASE_VERSION>/deployments/operator/samples/deviceplugin_v1_sgxdeviceplugin.yaml'
```

### Installation Using kubectl

There are two alternative ways to deploy SGX device plugin using `kubectl`.

The first approach involves deployment of the [SGX DaemonSet YAML](/deployments/sgx_plugin/base/intel-sgx-plugin.yaml)
and [node-feature-discovery](/deployments/nfd/kustomization.yaml)
with the necessary configuration.

The following kustomizations are needed for deploying everything:
```bash
# first, deploy NFD and the necessary NodeFeatureRules
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd'
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules'
# and then, deploy SGX plugin
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_plugin/overlays/epc-nfd/'
```

The second approach has a lesser deployment footprint. It does not require NFD, but a helper daemonset that creates `sgx.intel.com/capable='true'` node label and advertises EPC capacity directly to the API server.

The following kustomization is used for this approach:
```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_plugin/overlays/epc-register/
```

Additionally, SGX admission webhook must be deployed
```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_admissionwebhook/
# or when cert-manager is used
$ https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_admissionwebhook/overlays/default-with-certmanager
```

### Verify Plugin Registration

Verification of the plugin deployment and detection of SGX hardware can be confirmed by
examining the resource allocations on the nodes:

```bash
$ kubectl describe node <node name> | grep sgx.intel.com
                   nfd.node.kubernetes.io/extended-resources: sgx.intel.com/epc
 sgx.intel.com/enclave:    20
 sgx.intel.com/epc:        98566144
 sgx.intel.com/provision:  20
 sgx.intel.com/enclave:    20
 sgx.intel.com/epc:        98566144
 sgx.intel.com/provision:  20
 sgx.intel.com/enclave    0           0
 sgx.intel.com/epc        0           0
 sgx.intel.com/provision  0           0
```

## Testing and Demos
### SGX ECDSA Remote Attestation

The SGX remote attestation allows a relying party to verify that the software is running inside an Intel® SGX enclave on a platform
that has the trusted computing base up to date.

The demo guides to run an SGX DCAP/ECDSA quote generation in on a single-node kubernetes cluster using Intel® reference
SGX PCK Certificate Cache Service (PCCS) that is configured to service localhost connections.

Read more about [SGX Remote Attestation](https://software.intel.com/content/www/us/en/develop/topics/software-guard-extensions/attestation-services.html).

#### Remote Attestation Prerequisites

For the SGX ECDSA Remote Attestation demo to work, the platform must be correctly registered and a PCCS running.

For documentation to set up Intel® reference PCCS, refer to:
[Intel® Software Guard Extensions (Intel® SGX) Services](https://api.portal.trustedservices.intel.com/) and
[Intel® Software Guard Extensions SDK for Linux](https://www.intel.com/content/www/us/en/developer/tools/software-guard-extensions/overview.html)

Furthermore, the Kubernetes cluster must be set up according the [instructions above](#deploying-with-pre-built-images).

#### Build the image

The demo uses container images build from Intel® SGX SDK and DCAP releases.

To build the demo images:

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make sgx-aesmd-demo
...
Successfully tagged intel/sgx-aesmd-demo:devel
$ make sgx-sdk-demo
...
Successfully tagged intel/sgx-sdk-demo:devel
```

#### Deploy the pods

The demo runs Intel aesmd (architectural enclaves service daemon) that is responsible
for generating SGX quotes for workloads.

**Note**: The PCCS URL must be configured in `sgx_default_qcnl.conf`. The default `localhost` URL
is not available in containers

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_aesmd?ref=<RELEASE_VERSION>'
$ kubectl get pods
  NAME                     READY     STATUS    RESTARTS   AGE
  intel-sgx-aesmd-mrnm8                1/1     Running   0          3h47m
  sgxdeviceplugin-sample-z5dcq-llwlw   1/1     Running   0          28m
```
> **Note**: For quick experiments, [kind](https://kind.sigs.k8s.io/docs/user/configuration/) can be used to deploy the cluster. With `kind`, host path `/var/run/aesmd/` must be mounted to the nodes manually using [Extra Mounts](https://kind.sigs.k8s.io/docs/user/configuration/#extra-mounts`). \
> Example:
>```
>$ cat kind_config.yaml
>kind: Cluster
>apiVersion: kind.x-k8s.io/v1alpha4
>name: <your_node_name>
>nodes:
>- role: control-plane
>   extraMounts:
>   - hostPath: /var/run/aesmd
>     containerPath: /var/run/aesmd
>     propagation: Bidirectional
>```
> And bootstrap kind with it \
> `$ kind create cluster --config kind_config.yaml`

The sample application runs SGX DCAP Quote Generation sample:

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_enclave_apps/overlays/sgx_ecdsa_aesmd_quote?ref=<RELEASE_VERSION>'
$ kubectl get pods
  NAME                                 READY   STATUS      RESTARTS   AGE
  intel-sgx-aesmd-mrnm8                1/1     Running     0          3h55m
  ecdsa-quote-intelsgx-demo-job-vtq84  0/1     Completed   0          4s
  sgxdeviceplugin-sample-z5dcq-llwlw   1/1     Running     0          35m
$ kubectl logs ecdsa-quote-intelsgx-demo-job-vtq84

  Step1: Call sgx_qe_get_target_info:succeed!
  Step2: Call create_app_report:succeed!
  Step3: Call sgx_qe_get_quote_size:succeed!
  Step4: Call sgx_qe_get_quote:succeed!cert_key_type = 0x5
```

Similarly, full SGX DCAP Flow with Quote Generation and Trusted Quote Verification can be deployed using the `sgx_ecdsa_inproc_quote` overlay. Again, the PCCS URL must be set beforehand.

> **Note**: The deployment example above uses [kustomize](https://github.com/kubernetes-sigs/kustomize)
> that is available in kubectl since Kubernetes v1.14 release.
