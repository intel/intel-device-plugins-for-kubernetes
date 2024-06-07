# Intel GPU Level-Zero sidecar

Table of Contents

* [Introduction](#introduction)
* [Install](#install)

## Introduction

Intel GPU Level-Zero sidecar is an extension for the Intel GPU plugin that allows querying GPU details from the oneAPI/Level-Zero API. As the Level-Zero is a C/C++ API, it is preferred to keep the original GPU plugin as-is and add the additional functionality via the Level-Zero sidecar. The GPU plugin can be configured to use the Level-Zero sidecar with an overlay, see [install](#install).

Intel GPU plugin and the Level-Zero sidecar communicate via gRPC on a local socket visible only to the containers.

## Modes and Configuration Options

| Flag | Argument | Default | Meaning |
|:---- |:-------- |:------- |:------- |
| -socket | unix socket path | /var/lib/levelzero/server.sock | Unix socket path which the server registers itself into. |
| -v | verbosity | 1 | Log verbosity |

## Install

Installing the sidecar along with the GPU plugin happens via two possible overlays: [levelzero](../../deployments/gpu_plugin/overlays/levelzero/) and [wsl](../../deployments/gpu_plugin/overlays/wsl/).

Levelzero overlay adds the sidecar to the base GPU plugin deployment:

```bash
$ kubectl -k deployments/gpu_plugin/overlays/levelzero
```

WSL adds another layer on top of the Levelzero. WSL enables Intel GPU detection with WSL (Windows Subsystem for Linux) Kubernetes clusters:

```bash
$ kubectl -k deployments/gpu_plugin/overlays/wsl
```
