# Intel FPGA OCI createRuntime hook for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Dependencies](#dependencies)
* [Configuring CRI runtimes](#configuring-cri-runtimes)

## Introduction

The FPGA CDI hook is one of the components used to add support for Intel FPGA
devices to Kubernetes.

The FPGA OCI createRuntime hook is passed by the [FPGA device plugin](../fpga_plugin/README.md) as
a CDI device attribute to the Kubelet and then to the CRI runtime.
It performs discovery of the requested FPGA function bitstream and then programs FPGA devices
based on the environment variables in the workload description.

The hook is only *required* when the [FPGA admission webhook](../fpga_admissionwebhook/README.md)
is configured for orchestration programmed mode, and is benign (un-used) otherwise.

> **Note:** The fpga CDI hook is usually installed by the same DaemonSet as the
> FPGA device plugin. If building and installing the hook by hand, it is
> recommended you reference the
> [fpga plugin DaemonSet YAML](/deployments/fpga_plugin/base/intel-fpga-plugin-daemonset.yaml ) for
> more details.

## Dependencies

This component is one of a set of components that work together. You may also want to
install the following:

-   [FPGA device plugin](../fpga_plugin/README.md)
-   [FPGA admission controller](../fpga_admissionwebhook/README.md)

All components have the same basic dependencies as the
[generic plugin framework dependencies](../../README.md#about)

See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the CRI hook.

## Configuring CRI runtimes

CDI should be enabled for the CRI runtime to call the hook. CRI-O has it enabled by
default and for Containerd it should be enabled explicitly in its configuration file as
explained in the [CDI documentation](https://github.com/cncf-tags/container-device-interface?tab=readme-ov-file#how-to-configure-cdi)
