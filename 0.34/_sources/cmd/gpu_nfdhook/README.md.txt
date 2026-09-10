# Intel GPU NFD hook

Table of Contents

* [Introduction](#introduction)
* [GPU memory](#gpu-memory)
* [Default labels](#default-labels)
* [PCI-groups (optional)](#pci-groups-optional)
* [Capability labels (optional)](#capability-labels-optional)
* [Limitations](#limitations)

## Introduction

***NOTE:*** NFD's binary hook support will be turned off by default in the 0.14 release. The functionality in the GPU NFD hook is moved into a new NFD rule and into GPU plugin, and the capability labels are being removed completely. The GPU plugin deployment doesn't anymore support using init container. This directory will be removed in the future.

This is the [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)
binary hook implementation for the Intel GPUs. The intel-gpu-initcontainer (which
is built with the other images) can be used as part of the gpu-plugin deployment
to copy hook to the host systems on which gpu-plugin itself is deployed.

When NFD worker runs this hook, it will add a number of labels to the nodes,
which can be used for example to deploy services to nodes with specific GPU
types. Selected numeric labels can be turned into kubernetes extended resources
by the NFD, allowing for finer grained resource management for GPU-using PODs.

In the NFD deployment, the hook requires `/host-sys` -folder to have the host `/sys`-folder content mounted. Write access is not necessary.

For detailed info about the labels created by the NFD hook, see the [labels documentation](../gpu_plugin/labels.md).
