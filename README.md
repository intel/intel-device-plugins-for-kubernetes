# ![Intel Device Plugins for Kubernetes](.intel-logo.png) Device Plugins for Kubernetes
[![Build Status](https://travis-ci.org/intel/intel-device-plugins-for-kubernetes.svg?branch=master)](https://travis-ci.org/intel/intel-device-plugins-for-kubernetes)
[![Go Report Card](https://goreportcard.com/badge/github.com/intel/intel-device-plugins-for-kubernetes)](https://goreportcard.com/report/github.com/intel/intel-device-plugins-for-kubernetes)

## Table of Contents

- [About](#about)
- [GPU device plugin](cmd/gpu_plugin/README.md)
- FPGA plugin code
    - [FPGA device plugin](cmd/fpga_plugin/README.md)
    - [FPGA admission controller webhook](cmd/fpga_admissionwebhook/README.md)
    - [FPGA prestart CRI-O hook](cmd/fpga_crihook/README.md)
- [QuickAssist device plugin](cmd/qat_plugin/README.md)
- [How to develop simple device plugins](DEVEL.md)

## About

This repository contains a set of Kubernetes plugins that enable you to use
Intel devices.

Prerequisites for building and running Intel device plugins include:

- Intel hardware
- Fully configured [Kubernetes cluster]
- Working [Go environment]

## Supported Kubernetes versions

| Branch            | Kubernetes branch/version      |
|-------------------|--------------------------------|
| release-0.11      | Kubernetes 1.11 branch v1.11.x |

[Go environment]: https://golang.org/doc/install
[Kubernetes cluster]: https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/
