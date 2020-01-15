# ![Intel Device Plugins for Kubernetes](.intel-logo.png) Device Plugins for Kubernetes
[![Build Status](https://travis-ci.org/intel/intel-device-plugins-for-kubernetes.svg?branch=master)](https://travis-ci.org/intel/intel-device-plugins-for-kubernetes)
[![Go Report Card](https://goreportcard.com/badge/github.com/intel/intel-device-plugins-for-kubernetes)](https://goreportcard.com/report/github.com/intel/intel-device-plugins-for-kubernetes)

## Table of Contents

- [About](#about)
- [Prerequisites](#prerequisites)
- [GPU device plugin](cmd/gpu_plugin/README.md)
- FPGA plugin code
    - [FPGA device plugin](cmd/fpga_plugin/README.md)
    - [FPGA admission controller webhook](cmd/fpga_admissionwebhook/README.md)
    - [FPGA prestart CRI-O hook](cmd/fpga_crihook/README.md)
- [QuickAssist device plugin](cmd/qat_plugin/README.md)
- [How to develop simple device plugins](DEVEL.md)

## About

This repository contains a framework for developing plugins for the Kubernetes
[device plugins framework](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/),
along with a number of device implementations utilising that framework.

## Prerequisites

Prerequisites for building and running Intel device plugins include:

- Appropriate hardware
- A fully configured [Kubernetes cluster]
- A working [Go environment]

## Supported Kubernetes versions

| Branch            | Kubernetes branch/version      |
|-------------------|--------------------------------|
| release-0.15      | Kubernetes 1.15 branch v1.15.x |
| release-0.11      | Kubernetes 1.11 branch v1.11.x |

[Go environment]: https://golang.org/doc/install
[Kubernetes cluster]: https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/
