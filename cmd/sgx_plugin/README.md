# Intel Software Guard Extensions (SGX) device plugin for Kubernetes

Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
        * [Backwards compatiblity note](#backwards-compatibility-note)
    * [Pre-built images](#pre-built-images)
    * [Getting the source code](#getting-the-source-code)
    * [Verify node kubelet config](#verify-node-kubelet-config)
    * [Deploying as a DaemonSet](#deploying-as-a-daemonset)
        * [Build the plugin image](#build-the-plugin-image)
        * [Deploy the DaemonSet](#deploy-the-daemonset)
        * [Verify SGX device plugin is registered](#verify-sgx-device-plugin-is-registered)
    * [Deploying by hand](#deploying-by-hand)
        * [Build SGX device plugin](#build-sgx-device-plugin)
        * [Deploy SGX plugin](#deploy-sgx-plugin)
    * [SGX device plugin demos](#sgx-device-plugin-demos)
        * [SGX ECDSA Remote Attestation](#sgx-ecdsa-remote-attestation)
            * [Remote Attestation Prerequisites](#remote-attestation-prerequisites)
            * [Build the images](#build-the-image)
            * [Deploy the pod](#deploy-the-pod)

## Introduction

The Intel SGX device plugin and related components allow workloads to use Intel SGX on
platforms with SGX Flexible Launch Control enabled, e.g.,:

- 3rd Generation Intel® Xeon® Scalable Platform, code-named “Ice Lake”
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
| -enclave-limit | int | the number of containers per node allowed to use `/dev/sgx/enclave` (default: `20`) |
| -provision-limit | int | the number of containers per node allowed to use `/dev/sgx/provision` (default: `20`) |

The plugin also accepts a number of other arguments related to logging. Please use the `-h` option to see
the complete list of logging related options.

## Installation

The following sections cover how to obtain, build and install the necessary Kubernetes SGX specific
components.

They can be installed either using a DaemonSet or running 'by hand' on each node.

### Prerequisites

The component has the same basic dependancies as the
[generic plugin framework dependencies](../../README.md#about).

The SGX device plugin requires Linux Kernel SGX drivers to be available. These drivers
are available in Linux 5.11.

The hardware platform must support SGX Flexible Launch Control.

#### Backwards compatibility note

The SGX device nodes have changed from `/dev/sgx/[enclave|provision]`
to `/dev/sgx_[enclave|provision]` in v4x RFC patches according to the
LKML feedback.

Backwards compatibility is provided by adding `/dev/sgx` directory volume
mount to containers. This assumes the cluster admin has installed the
udev rules provided below to make the old device nodes as symlinks to the
new device nodes.

**Note:** the symlinks become visible in all containers requesting SGX
resources but are potentially dangling links if the device the corresponding
device resource is not requested.

```bash
$ cat /etc/udev/rules/9*.rules
SUBSYSTEM=="misc",KERNEL=="enclave",MODE="0666"
SUBSYSTEM=="misc",KERNEL=="sgx_enclave",MODE="0666",SYMLINK+="sgx/enclave"
SUBSYSTEM=="sgx",KERNEL=="sgx/enclave",MODE="0666"
SUBSYSTEM=="misc",KERNEL=="provision",MODE="0660"
SUBSYSTEM=="misc",KERNEL=="sgx_provision",SYMLINK+="sgx/provision",MODE="0660"
SUBSYSTEM=="sgx",KERNEL=="sgx/provision",MODE="0660"
$ sudo udevadm trigger
$ ls -la /dev/sgx/*
lrwxrwxrwx 1 root root 14 Nov 18 01:01 /dev/sgx/enclave -> ../sgx_enclave
lrwxrwxrwx 1 root root 16 Nov 18 01:01 /dev/sgx/provision -> ../sgx_provision
```

The backwards compatibility will be removed in the next release (v0.20) and
from the main development branch once the SGX SDK and DCAP releases default to
the new devices.

### Pre-built images

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

`<RELEASE_VERSION>` needs to be substituted with the desired release version, e.g. `v0.19.0` or main.

#### Deploy node-feature-discovery

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_nfd?ref=<RELEASE_VERSION>
```

#### Deploy cert-manager

```bash
$ kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.2.0/cert-manager.yaml
```

#### Deploy Intel Device plugin operator

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/operator/default?ref=<RELEASE_VERSION>
```

**Note:** See the operator [deployment details](/cmd/operator/README.md) for setting it up on systems behind proxies.

#### Deploy SGX device plugin with the operator

```bash
$ kubectl apply -f https://raw.githubusercontent.com/intel/intel-device-plugins-for-kubernetes/main/deployments/operator/samples/deviceplugin_v1_sgxdeviceplugin.yaml
```

### Getting the source code

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Deploying as a DaemonSet

To deploy the plugin as a DaemonSet, you first need to build a container image for the plugin and
ensure that is visible to your nodes.

#### Build the plugin and EPC source images

The following will use `docker` to build a local container images called `intel/intel-sgx-plugin`
and `intel/intel-sgx-initcontainer` with the tag `devel`. The image build tool can be changed from the
default docker by setting the `BUILDER` argument to the [Makefile](/Makefile).

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make intel-sgx-plugin
...
Successfully tagged intel/intel-sgx-plugin:devel
$ make intel-sgx-initcontainer
...
Successfully tagged intel/intel-sgx-initcontainer:devel
```

#### Deploy the DaemonSet

Deploying the plugin involves the deployment of the
[SGX DaemonSet YAML](/deployments/sgx_plugin/base/intel-sgx-plugin.yaml)
and [node-feature-discovery](/deployments/sgx_nfd/kustomization.yaml)
with the necessary configuration.

There is a kustomization for deploying everything:
```bash
$ kubectl apply -k ${INTEL_DEVICE_PLUGINS_SRC}/deployments/sgx_plugin/overlays/epc-nfd/
```

#### Verify SGX device plugin is registered:

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
 sgx.intel.com/enclave    1           1
 sgx.intel.com/epc        400         400
 sgx.intel.com/provision  1           1
```

### Deploying by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

#### Build SGX device plugin

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make sgx_plugin
```

#### Deploy SGX plugin

Deploy the plugin on a node by running it as `root`. The below is just an example - modify the
paramaters as necessary for your setup:

```bash
$ sudo -E ${INTEL_DEVICE_PLUGINS_SRC}/cmd/sgx_plugin/sgx_plugin -enclave-limit 50 -provision-limit 1 -v 2
I0626 20:33:01.414446  964346 server.go:219] Start server for provision at: /var/lib/kubelet/device-plugins/sgx.intel.com-provision.sock
I0626 20:33:01.414640  964346 server.go:219] Start server for enclave at: /var/lib/kubelet/device-plugins/sgx.intel.com-enclave.sock
I0626 20:33:01.417315  964346 server.go:237] Device plugin for provision registered
I0626 20:33:01.417748  964346 server.go:237] Device plugin for enclave registered
```

### SGX device plugin demos
#### SGX ECDSA Remote Attestation

The SGX remote attestation allows a relying party to verify that the software is running inside an Intel® SGX enclave on a platform
that has the trusted computing base up to date.

The demo guides to run an SGX DCAP/ECDSA quote generation in on a single-node kubernetes cluster using Intel® reference
SGX PCK Certificate Cache Service (PCCS) that is configured to service localhost connections.

Read more about [SGX Remote Attestation](https://software.intel.com/content/www/us/en/develop/topics/software-guard-extensions/attestation-services.html).

##### Remote Attestation Prerequisites

For the SGX ECDSA Remote Attestation demo to work, the platform must be correctly registered and a PCCS running.

For documentation to set up Intel® reference PCCS, refer to:
[Intel® Software Guard Extensions (Intel® SGX) Services](https://api.portal.trustedservices.intel.com/) and
[Intel® Software Guard Extensions SDK for Linux](https://01.org/intel-software-guard-extensions)

Furthermore, the Kubernetes cluster must be set up according the [instructions above](#pre-built-images).

##### Build the image

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

##### Deploy the pods

The demo runs Intel aesmd (architectural enclaves service daemon) that is responsible
for generating SGX quotes for workloads. It is deployed with `hostNetwork: true`
to allow connections to localhost PCCS.

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_aesmd?ref=<RELEASE_VERSION>
$ kubectl get pods
  NAME                     READY     STATUS    RESTARTS   AGE
  intel-sgx-aesmd-mrnm8                1/1     Running   0          3h47m
  sgxdeviceplugin-sample-z5dcq-llwlw   1/1     Running   0          28m
```

The sample application runs SGX DCAP Quote Generation sample:

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/sgx_enclave_apps/overlays/sgx_ecdsa_aesmd_quote?ref=<RELEASE_VERSION>
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

> **Note**: The deployment example above uses [kustomize](https://github.com/kubernetes-sigs/kustomize)
> that is available in kubectl since Kubernetes v1.14 release.
