# Intel Software Guard Extensions (SGX) device plugin for Kubernetes

# Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
    * [Getting the source code:](#getting-the-source-code)
    * [Verify node kubelet config](#verify-node-kubelet-config)
    * [Deploying as a DaemonSet](#deploying-as-a-daemonset)
        * [Build the plugin image](#build-the-plugin-image)
        * [Deploy the DaemonSet](#deploy-the-daemonset)
        * [Verify SGX device plugin is registered on master:](#verify-sgx-device-plugin-is-registered-on-master)
    * [Deploying by hand](#deploying-by-hand)
        * [Build SGX device plugin](#build-sgx-device-plugin)
        * [Deploy SGX plugin](#deploy-sgx-plugin)

# Introduction

**Note:** The work is still WIP. The SGX device plugin can be tested to run simple enclaves
but the full e2e deployment (including the SGX remote attestation) is not yet finished. See
the open issues for details.

This Intel SGX device plugin provides support for Intel SGX TEE under Kubernetes.

## Modes and Configuration options

The SGX plugin can take a number of command line arguments, summarised in the following table:

| Flag | Argument | Meaning |
|:---- |:-------- |:------- |
| -enclave-limit | int | the number of containers per node allowed to use `/dev/sgx/enclave` (default: `20`) |
| -provision-limit | int | the number of containers per node allowed to use `/dev/sgx/provision` (default: `20`) |

The plugin also accepts a number of other arguments related to logging. Please use the `-h` option to see
the complete list of logging related options.

# Installation

The below sections cover how to obtain, build and install this component.

The component can be installed either using a DaemonSet or running 'by hand' on each node.

## Prerequisites

The component has the same basic dependancies as the
[generic plugin framework dependencies](../../README.md#about).

The SGX plugin requires Linux Kernel SGX drivers to be available. These drivers
are currently available via RFC patches on Linux Kernel Mailing List.

## Getting the source code

```bash
$ mkdir -p $(go env GOPATH)/src/github.com/intel
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
```

## Verify node kubelet config

Every node that will be running the plugin must have the
[kubelet device-plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
configured. For each node, check that the kubelet device plugin socket exists:

```bash
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

## Deploying as a DaemonSet

To deploy the plugin as a DaemonSet, you first need to build a container image for the plugin and
ensure that is visible to your nodes.

### Build the plugin and EPC source images

The following will use `docker` to build a local container images called `intel/intel-sgx-plugin`
and `intel/intel-sgx-initcontainer` with the tag `devel`. The image build tool can be changed from the
default docker by setting the `BUILDER` argument to the [Makefile](../../Makefile).

```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make intel-sgx-plugin
...
Successfully tagged intel/intel-sgx-plugin:devel
$ make intel-sgx-initcontainer
...
Successfully tagged intel/intel-sgx-initcontainer:devel
```

### Deploy the DaemonSet

Deploying the plugin involves the deployment of a
[NFD EPC Source InitContainer Job](../../deployments/sgx_plugin/base/intel-sgx-hookinstall.yaml) the
[DaemonSet YAML](../../deployments/sgx_plugin/base/intel-sgx-plugin.yaml), and node-feature-discovery
with the necessary configuration.

There is a kustomization for deploying everything:
```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ kubectl apply -k deployments/sgx_plugin/overlays/nfd
```

### Verify SGX device plugin is registered on master:

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

## Deploying by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

### Build SGX device plugin

```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make sgx_plugin
```

### Deploy SGX plugin

Deploy the plugin on a node by running it as `root`. The below is just an example - modify the
paramaters as necessary for your setup:

```bash
$ sudo $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/sgx_plugin/sgx_plugin \
-enclave-limit 50 -provision-limit 1 -v 2
I0626 20:33:01.414446  964346 server.go:219] Start server for provision at: /var/lib/kubelet/device-plugins/sgx.intel.com-provision.sock
I0626 20:33:01.414640  964346 server.go:219] Start server for enclave at: /var/lib/kubelet/device-plugins/sgx.intel.com-enclave.sock
I0626 20:33:01.417315  964346 server.go:237] Device plugin for provision registered
I0626 20:33:01.417748  964346 server.go:237] Device plugin for enclave registered
```
