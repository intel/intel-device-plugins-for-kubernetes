# Overview
[![Build Status](https://github.com/intel/intel-device-plugins-for-kubernetes/workflows/CI/badge.svg?branch=main)](https://github.com/intel/intel-device-plugins-for-kubernetes/actions?query=workflow%3ACI)
[![Go Report Card](https://goreportcard.com/badge/github.com/intel/intel-device-plugins-for-kubernetes)](https://goreportcard.com/report/github.com/intel/intel-device-plugins-for-kubernetes)
[![GoDoc](https://godoc.org/github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin?status.svg)](https://godoc.org/github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin)

This repository contains a framework for developing plugins for the Kubernetes
[device plugins framework](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/),
along with a number of device plugin implementations utilising that framework.

The [v0.19 release](https://github.com/intel/intel-device-plugins-for-kubernetes/releases/latest)
is the latest feature release with its documentation available [here](https://intel.github.io/intel-device-plugins-for-kubernetes/0.19/).

Table of Contents

* [Prerequisites](#prerequisites)
* [Plugins](#plugins)
    * [GPU device plugin](#gpu-device-plugin)
    * [FPGA device plugin](#fpga-device-plugin)
    * [QAT device plugin](#qat-device-plugin)
    * [VPU device plugin](#vpu-device-plugin)
    * [SGX device plugin](#sgx-device-plugin)
    * [DSA device pugin](#dsa-device-plugin)
* [Device Plugins Operator](#device-plugins-operator)
* [Demos](#demos)
* [Developers](#developers)
* [Running e2e Tests](#running-e2e-tests)
* [Supported Kubernetes versions](#supported-kubernetes-versions)
* [Pre-built plugin images](#pre-built-plugin-images)
* [License](#license)
* [Security](#security)
* [Related code](#related-code)

## Prerequisites

Prerequisites for building and running these device plugins include:

- Appropriate hardware
- A fully configured [Kubernetes cluster]
- A working [Go environment], of at least version v1.15.

## Plugins

The below sections detail existing plugins developed using the framework.

### GPU device plugin

The [GPU device plugin](cmd/gpu_plugin/README.md) supports Intel
[GVT-d](https://github.com/intel/gvt-linux/wiki/GVTd_Setup_Guide) device passthrough
and acceleration using GPUs of the following hardware families:

- Integrated GPUs within Intel Core processors
- Intel Xeon processors
- Intel Visual Compute Accelerator (Intel VCA)

The demo subdirectory contains both a [GPU plugin demo video](demo/readme.md#intel-gpu-device-plugin-demo-video)
as well as code for an OpenCL [FFT demo](demo/ubuntu-demo-opencl).

### FPGA device plugin

The [FPGA device plugin](cmd/fpga_plugin/README.md) supports FPGA passthrough for
the following hardware:

- Intel Arria 10
- Intel Stratix 10

The FPGA plugin comes as three parts.

- the [device plugin](#device-plugin)
- the [admission controller](#admission-controller)
- the [CRIO-O prestart hook](#cri-o-prestart-hook)

Refer to each individual sub-components documentation for more details. Brief overviews
of the sub-components are below.

The demo subdirectory contains a [video](demo/readme.md#intel-fpga-device-plugin-demo-video) showing deployment
and use of the FPGA plugin. Sources relating to the demo can be found in the
[opae-nlb-demo](demo/opae-nlb-demo) subdirectory.

#### device plugin

The [FPGA device plugin](cmd/fpga_plugin/README.md) is responsible for discovering and reporting FPGA
devices to `kubelet`.

#### admission controller

The [FPGA admission controller webhook](cmd/fpga_admissionwebhook/README.md) is responsible for performing
mapping from user-friendly function IDs to the Interface ID and Bitstream ID that are required for FPGA
programming.  It also implements access control by namespacing FPGA configuration information.

#### CRI-O prestart hook

The [FPGA prestart CRI-O hook](cmd/fpga_crihook/README.md) performs discovery of the requested FPGA
function bitstream and programs FPGA devices based on the environment variables in the workload
description.

### [QAT](https://01.org/intel-quick-assist-technology) device plugin

The [QAT plugin](cmd/qat_plugin/README.md) supports device plugin for Intel QAT adapters, and includes
code [showing deployment](cmd/qat_plugin/dpdkdrv) via [DPDK](https://doc.dpdk.org/guides/cryptodevs/qat.html).

The demo subdirectory includes details of both a
[QAT DPDK demo](demo/readme.md#intel-quickassist-technology-device-plugin-with-dpdk-demo-video)
and a [QAT OpenSSL demo](demo/readme.md#intel-quickassist-technology-device-plugin-openssl-demo-video).
Source for the OpenSSL demo can be found in the [relevant subdirectory](demo/openssl-qat-engine).

Details for integrating the QAT device plugin into [Kata Containers](https://katacontainers.io/)
can be found in the
[Kata Containers documentation repository](https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md).

### VPU device plugin

The [VPU device plugin](cmd/vpu_plugin/README.md) supports Intel VCAC-A card
(https://www.intel.com/content/dam/www/public/us/en/documents/datasheets/media-analytics-vcac-a-accelerator-card-by-celestica-datasheet.pdf)
the card has:
- 1 Intel Core i3-7100U processor
- 12 MyriadX VPUs
- 8GB DDR4 memory

The demo subdirectory includes details of a OpenVINO deployment and use of the VPU plugin.
Sources can be found in [openvino-demo](demo/ubuntu-demo-openvino)

### SGX device plugin

The [SGX device plugin](cmd/sgx_plugin/README.md) allows workloads to use Intel SGX on
platforms with SGX Flexible Launch Control enabled, e.g.,:

- 3rd Generation Intel® Xeon® Scalable Platform, code-named “Ice Lake”
- Intel® Xeon® E3
- Intel® NUC Kit NUC7CJYH

The SGX plugin comes in three parts.

- the [device plugin](#sgx-plugin)
- the [admission webhook](#sgx-admission-webhook)
- the [SGX EPC memory registration](#sgx-epc-memory-registration)

The demo subdirectory contains a [video](demo/readme.md#intel-sgx-device-plugin-demo-video) showing the deployment
and use of the SGX device plugin. Sources relating to the demo can be found in the
[sgx-sdk-demo](demo/sgx-sdk-demo) and [sgx-aesmd-demo](demo/sgx-aesmd-demo) subdirectories.

Brief overviews of the SGX sub-components are given below.

<a name="sgx-plugin"></a>
#### device plugin

The [SGX device plugin](cmd/sgx_plugin/README.md) is responsible for discovering and reporting SGX
device nodes to `kubelet`.

Containers requesting SGX resources in the cluster should not use the device plugins resources directly.

#### SGX Admission webhook

The SGX admission webhook is responsible for performing Pod mutations based on the `sgx.intel.com/quote-provider`
pod annotation set by the user. The purpose of the webhook is to hide the details of setting the necessary
device resources and volume mounts for using SGX remote attestation in the cluster. Furthermore,
the SGX admission webhook is responsible for writing a pod/sandbox `sgx.intel.com/total_epc` annotation that
is used by Kata Containers to dynamically adjust its virtualized SGX encrypted page cache (EPC) bank(s) size.

The SGX admission webhook is implemented as part of [Intel Device Plugin Operator](cmd/operator/README.md).

#### SGX EPC memory registration

The SGX EPC memory available on each node is registered as a Kubernetes extended resource using
node-feature-discovery (NFD). A custom NFD source hook is installed as part of [SGX device plugin](cmd/sgx_plugin/README.md)
operator deployment and NFD is configured to register the SGX EPC memory extended resource reported by the hook.

Containers requesting SGX EPC resources in the cluster use `sgx.intel.com/epc` resource which is of
type [memory](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory).

### DSA device plugin

The [DSA device plugin](cmd/dsa_plugin/README.md) supports acceleration using the Intel Data Streaming accelerator(DSA).

## Device Plugins Operator

Currently the operator has support for the QAT, GPU, FPGA, SGX and DSA device plugins:
it validates container image references and extends reported statuses.

To run an operator instance in the container run

```bash
$ kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.2.0/cert-manager.yaml
$ make deploy-operator
```

Then deploy your device plugin by applying its custom resource, e.g.
`GpuDevicePlugin` with

```bash
$ kubectl apply -f ./deployments/operator/samples/deviceplugin_v1_gpudeviceplugin.yaml
```

Observe it is up and running:

```bash
$ kubectl get GpuDevicePlugin
NAME                     DESIRED   READY   NODE SELECTOR   AGE
gpudeviceplugin-sample   1         1                       5s
```

## Demos

The [demo subdirectory](demo/readme.md) contains a number of demonstrations for a variety of the
available plugins.

## Developers

For information on how to develop a new plugin using the framework, see the
[Developers Guide](DEVEL.md) and the code in the
[device plugins pkg directory](pkg/deviceplugin).

## Running E2E tests

Currently the E2E tests require having a Kubernetes cluster already configured
on the nodes with the hardware required by the device plugins. Also all the
container images with the executables under test must be available in the
cluster. Given these two conditions are satisfied one can run the tests with

```bash
$ go test -v ./test/e2e/...
```

In case you want to run only certain tests, e.g. QAT ones, then run

```bash
$ go test -v ./test/e2e/... -args -ginkgo.focus "QAT"
```

If you need to specify paths to your custom `kubeconfig` containing
embedded authentication info then add the `-kubeconfig` argument:

```bash
$ go test -v ./test/e2e/... -args -kubeconfig /path/to/kubeconfig
```

The full list of available options can be obtained with

```bash
$ go test ./test/e2e/... -args -help
```

Also it is possible to run the tests which don't depend on hardware
without a pre-configured Kubernetes cluster. Just make sure you have
[Kind](https://kind.sigs.k8s.io/) installed on your host and run

```
$ make test-with-kind
```

## Running controller tests with a local control plane

The controller-runtime library provides a package for integration testing by
starting a local control plane. The package is called
[envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest). The
operator uses this package for its integration testing.
Please have a look at `envtest`'s documentation to set up it properly. But basically
you just need to have `etcd` and `kube-apiserver` binaries available on your
host. By default they are expected to be located at `/usr/local/kubebuilder/bin`.
But you can have it stored anywhere by setting the `KUBEBUILDER_ASSETS`
environment variable. So, given you have the binaries copied to
`${HOME}/work/kubebuilder-assets` to run the tests just enter

```bash
$ KUBEBUILDER_ASSETS=${HOME}/work/kubebuilder-assets make envtest
```

## Supported Kubernetes versions

Releases are made under the github [releases area](https://github.com/intel/intel-device-plugins-for-kubernetes/releases). Supported releases and
matching Kubernetes versions are listed below:

| Branch            | Kubernetes branch/version      | Status      |
|:------------------|:-------------------------------|:------------|
| release-0.19      | Kubernetes 1.19 branch v1.19.x | supported   |
| release-0.18      | Kubernetes 1.18 branch v1.18.x | supported   |
| release-0.17      | Kubernetes 1.17 branch v1.17.x | unsupported |
| release-0.15      | Kubernetes 1.15 branch v1.15.x | unsupported |
| release-0.11      | Kubernetes 1.11 branch v1.11.x | unsupported |

[Go environment]: https://golang.org/doc/install
[Kubernetes cluster]: https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/

## Pre-built plugin images

Pre-built images of the plugins are available on the Docker hub. These images are automatically built and uploaded to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their release version numbers in the format x.y.z, corresponding to the branches and releases in this repository.

**Note:** the default deployment files and operators are configured with [imagePullPolicy](https://kubernetes.io/docs/concepts/containers/images/#updating-images) ```IfNotPresent``` and can be changed with ```scripts/set-image-pull-policy.sh```.

## License

All of the source code required to build intel-device-plugins-for-kubernetes
is available under Open Source licenses. The source code files identify external Go
modules used. Binaries are distributed as container images on
DockerHub. Those images contain license texts under
`/usr/local/share/package-licenses` and source code under
`/usr/local/share/package-sources`.

## Security

**Reporting a Potential Security Vulnerability**: If you have discovered
potential security vulnerability in this project, please send an e-mail to
secure@intel.com. Encrypt sensitive information using our [PGP public key](https://www.intel.com/content/www/us/en/security-center/pgp-public-key.html).

Please provide as much information as possible, including:
  - The projects and versions affected
  - Detailed description of the vulnerability
  - Information on known exploits

A member of the Intel Product Security Team will review your e-mail and
contact you to collaborate on resolving the issue. For more information on
how Intel works to resolve security issues, see [Vulnerability Handling Guidelines](https://www.intel.com/content/www/us/en/security-center/vulnerability-handling-guidelines.html).

## Related code

A related Intel SRIOV network device plugin can be found in [this repository](https://github.com/intel/sriov-network-device-plugin)
