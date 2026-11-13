# Intel GPU Level-Zero sidecar

Table of Contents

* [Introduction](#introduction)
* [Install](#install)

## Introduction

Intel GPU Level-Zero sidecar is an extension for the Intel GPU plugin to query additional GPU details from the oneAPI/Level-Zero API. As the Level-Zero is a C/C++ API, it is preferred to keep the original GPU plugin as-is and add the additional functionality via the Level-Zero sidecar. The GPU plugin can be configured to use the Level-Zero sidecar with an overlay, see [install](#install).

Intel GPU plugin and the Level-Zero sidecar communicate via gRPC on a local socket visible only to the containers.

> **NOTE**: Intel Device Plugin Operator doesn't yet support enabling Level-Zero sidecar in the GPU CR object.

## Modes and Configuration Options

| Flag | Argument | Default | Meaning |
|:---- |:-------- |:------- |:------- |
| -socket | unix socket path | /var/lib/levelzero/server.sock | Unix socket path which the server registers itself into. |
| -wsl | - | disabled | Adapt sidecar to run in the WSL environment. |
| -v | verbosity | 1 | Log verbosity |

## Install

Installing the sidecar along with the GPU plugin happens via two possible overlays: [health](../../deployments/gpu_plugin/overlays/health/) and [wsl](../../deployments/gpu_plugin/overlays/wsl/).

Health overlay adds the sidecar to the base GPU plugin deployment and configures GPU plugin to retrieve device health indicators from the Level-Zero API:

```bash
$ kubectl -k deployments/gpu_plugin/overlays/health
```

WSL layer enables Intel GPU detection with WSL (Windows Subsystem for Linux) Kubernetes clusters. It also leverages the Level-Zero sidecar:

```bash
$ kubectl -k deployments/gpu_plugin/overlays/wsl
```
