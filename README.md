# ![Intel Device Plugins for Kubernetes](.intel-logo.png) Device Plugins for Kubernetes
[![Build Status](https://travis-ci.org/intel/intel-device-plugins-for-kubernetes.svg?branch=master)](https://travis-ci.org/intel/intel-device-plugins-for-kubernetes)
[![Go Report Card](https://goreportcard.com/badge/github.com/intel/intel-device-plugins-for-kubernetes)](https://goreportcard.com/report/github.com/intel/intel-device-plugins-for-kubernetes)

## Table of Contents

- [About](#about)
- [GPU device plugin](cmd/gpu_plugin/README.md)
- [FPGA device plugin](cmd/fpga_plugin/README.md)
- [FPGA admission controller webhook](cmd/fpga_admissionwebhook/README.md)
- [FPGA prestart CRI-O hook](cmd/fpga_crihook/README.md)

## About

This repository contains set of Intel Device Plugins that enable usage of Intel hardware in Kubernetes cluster.

These are prerequisites for building and running Intel Device Plugins:

- Intel hardware
- Fully [configured kubernetes cluster]
- Working [Go environment]

## Supported Kubernetes versions

| Branch            | Kubernetes branch/version      |
|-------------------|--------------------------------|
| master            | Kubernetes 1.10 branch v1.10.x |
| master            | Kubernetes master branch       |

[Go environment]: https://golang.org/doc/install
[Configured Kubernetes cluster]: https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/
