# Intel SGX prestart/poststop OCI hook for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Dependencies](#dependencies)
* [Building](#building)
    * [Getting the source code](#getting-the-source-code)
    * [Building the image](#building-the-image)
* [Configuring CRI-O](#configuring-cri-o)

## Introduction

The SGX CRI-O OCI hook is one of the components used to add support for Intel SGX
to Kubernetes.

The hook is triggered by container annotations, such as set by the
[SGX device plugin](../sgx_plugin/README.md). It writes the per container SGX EPC
values to eBPF maps. The name of the map where to write the limits is defined by the
annotation.

## Dependencies

This component is one of a set of components that work together. You may also want to
install the following:

-   [SGX device plugin](../sgx_plugin/README.md)
-   [SGX admission controller](../sgx_admissionwebhook/README.md)
-   [SGX skeleton loader](../../src/sgx-skeleton/README.md)

All components have the same basic dependencies as the
[generic plugin framework dependencies](../../README.md#about)

## Building

The following sections detail how to obtain, build and deploy the OCI hook.

### Getting the source code

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Building binary

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make sgx_oci_epchook
```

## Configuring CRI-O

Recent versions of [CRI-O](https://github.com/cri-o/cri-o) are shipped with default configuration
file that prevents CRI-O to discover and configure hooks automatically.
Please ensure that your `/etc/crio/crio.conf` parameter `hooks_dir` is either unset
(to enable default search paths for OCI hooks configuration) or contains the directory
`/etc/containers/oci/hooks.d`.

prestart hook writes to eBPF maps:
```
{
     "version": "1.0.0",
     "hook": {
        "path": "/usr/libexec/oci/hooks.d/sgx_oci_epchook"
     },
     "when": {
        "annotations": {
                "com\\.intel\\.sgx": "container_sgx_epc_limit"
        }
     },
     "stages": ["prestart"]
}
```

poststop hook deletes per-container values from the eBPF maps:
```
{
     "version": "1.0.0",
     "hook": {
        "path": "/usr/libexec/oci/hooks.d/sgx_oci_epchook",
        "args": ["sgx_oci_epchook", "-cleanup"]
     },
     "when": {
        "annotations": {
                "com\\.intel\\.sgx": "container_sgx_epc_limit"
        }
     },
     "stages": ["poststop"]
}
```
