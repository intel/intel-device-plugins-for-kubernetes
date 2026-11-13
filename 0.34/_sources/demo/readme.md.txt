# Demo

Table of Contents

- [Demo overview](#demo-overview)
- [Intel® GPU Device Plugin demo video](#intel-gpu-device-plugin-demo-video)
- [Intel® FPGA Device Plugin demo videos](#intel-fpga-device-plugin-demo-videos)
- [Intel® SGX Device Plugin demo videos](#intel-sgx-device-plugin-demo-videos)
- [Intel® QuickAssist Technology Device Plugin OpenSSL demo video](#intel-quickassist-technology-device-plugin-openssl-demo-video)
- [Intel® QuickAssist Technology Device Plugin with DPDK demo video](#intel-quickassist-technology-device-plugin-with-dpdk-demo-video)

## Demo overview

Acceleration of compute and data processing of workloads like video
transcoding, compression, and encryption is enabled in Kubernetes with
the [Device Plugin Framework](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/). This repository
contains a set of Kubernetes plugins and instructions to enable Intel devices
for the acceleration of your workloads orchestrated by Kubernetes.

The current list of supported Intel Device Plugins includes:

- [GPU Device Plugin](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/cmd/gpu_plugin/README.md) with support for [Intel® Graphics Technology](https://www.intel.com/content/www/us/en/architecture-and-technology/visual-technology/graphics-overview.html)
- [Intel® FPGA Device Plugin](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/cmd/fpga_plugin/README.md)
- [Intel® QuickAssist Technology (QAT) Device Plugin](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/cmd/qat_plugin/README.md)

We have included an example demo and configuration information for the Intel
Device Plugins for Kubernetes below. Please join us on the sig-node-rtk channel
on [kubernetes.slack.com](https://kubernetes.slack.com/) to ask questions,
contribute to the community, and learn about the work we are doing with
Kubernetes and the Device Plugin Framework.

## Intel® GPU Device Plugin demo video

The screencast demonstrates the deployment of the Intel® GPU Device Plugin for
Kubernetes including Kubeless Function as a Service (FaaS) media transcoder
JavaScript function. The media transcoding workload is scheduled on two different worker nodes.
Only one worker node has a GPU. The time difference in transcoding speed is captured.

### Demo platform configuration

- Hardware 2-nodes
    - 1x Virtual Machine on Intel® Xeon® E5-2687 CPU @ 3.0 GHz
    - 1x Intel® NUC KIT NUC6i7KYK (Skull Canyon) with Intel integrated GPU
- Software
    - Ubuntu* 18.04 (Kernel: 4.15.0-36-generic)
    - Kubernetes* 1.11
    - Docker* 18.3.1
    - Intel® GPU Device Plugin built from main branch

### Screencast

[![Intel® GPU Device Plugin demo](https://img.youtube.com/vi/sg31rV1FdQk/0.jpg)](https://youtu.be/sg31rV1FdQk)

## Intel® FPGA Device Plugin demo videos

The screencasts demonstrate the deployment of the Intel® FPGA Device Plugin for
Kubernetes.

- Demo 1 executes a native loopback 3 (NLB3) workload in preprogrammed mode
- Demo 2 executes NLB3 workload in orchestrated mode
- Demo 3 runs an [OpenCL workload](https://www.intel.com/content/www/us/en/programmable/support/support-resources/design-examples/design-software/opencl/matrix-multiplication.html) to do English letters recognition, and compares time used with and without FPGA to show the acceleration.

The demos begin with a fully [configured Kubernetes cluster](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/).

### Demo 1 and 2 (NLB3) platform configuration

- Hardware
    - 1-node, Intel(R) Xeon(R) CPU E5-2650 0 @ 2.00GHz
    - Total memory 62 GB
    - 2 x Intel® Arria® 10 GX FPGA Programmable Acceleration Card(PAC)
- Software
    - openSUSE Leap 15.1 (Kernel: 5.4.5-1.g47eef04-default)
    - Kubernetes* 1.17
    - CRI-O 1.13.1 (for orchestrated mode)
    - Docker 19.03.1 (for preprogrammed mode)
    - runc 1.0.0-rc8
    - Intel® FPGA Device Plugin built from main branch

### Demo 3 (OpenCL) platform configuration

- Hardware
    - Multi-node, FPGA host has 24 cores Intel(R) Xeon(R) Gold 6252N CPU @ 2.30GHz
    - Total memory 195 GB
    - Intel® FPGA Programmable Acceleration Card D5005
- Software
    - Centos 7 (Kernel: 3.10.0-1062.12.1.el7.x86_64) on worker node host
    - Kubernetes* 1.17
    - CRI-O 1.17.3
    - Intel® FPGA Device Plugin built from main branch

### Demo steps

1. Validate the status of the [Kubernetes cluster](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/).
2. Provision the [Intel® FPGA Device Plugin](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/cmd/fpga_plugin/README.md).
3. Create bitstream storage (for orchestrated mode only)
4. Run the NLB3 or OpenCL workload.

### Screencasts

- Intel® FPGA Device Plugin deployment in preprogrammed mode and NLB workload:

[<img src="https://asciinema.org/a/oIwOtM8hflsWTDu6UhNVS8401.svg" width="700">](https://asciinema.org/a/oIwOtM8hflsWTDu6UhNVS8401)

- Intel® FPGA Device Plugin deployment with orchestrated/region mode and NLB workload:

[<img src="https://asciinema.org/a/sUnLNwpazbUXKdpC66g09W5w1.svg" width="700">](https://asciinema.org/a/sUnLNwpazbUXKdpC66g09W5w1)

- Intel® FPGA Device Plugin deployment with orchestrated/region mode and OpenCL workload:

[<img src="https://asciinema.org/a/344184.svg" width="700">](https://asciinema.org/a/344184)

## Intel® SGX Device Plugin demo videos

This video demonstrates the Intel® Software Guard Extensions ECDSA Quote Generation in Kubernetes*

- Hardware
    - 1-node, 3rd Generation Intel® Xeon® Scalable Platform, code-named “Ice Lake”
- Software
    - 18.04.5 LTS
    - Kubernetes* 1.19
    - containerd 1.3.3
    - Intel® SGX Device Plugin built from main branch
    - Intel® SGX SDK 2.12 and DCAP 1.9
    - node-feature-discovery 0.6.0

### Demo steps

1. Validate the status of the [Kubernetes cluster](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/).
2. Provision [node-feature-discovery](https://github.com/kubernetes-sigs/node-feature-discovery).
3. Provision the [Intel® SGX Device Plugin](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/cmd/sgx_plugin/README.md) using [Intel® Device Plugin Operator](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/cmd/operator/README.md).
4. Check the SGX resources and labels are correctly registered.
5. Run Intel® SGX DCAP ECDSA Quote Generation in both "out-of-proc" and "in-proc" modes.

### Screencasts

Intel® SGX Device Plugin and SGX DCAP ECDSA Quote Generation demo
[<img src="https://asciinema.org/a/0xa888OjktVyz6xf0RSQ6Pi0w.svg" width="700">](https://asciinema.org/a/0xa888OjktVyz6xf0RSQ6Pi0w)

## Intel® QuickAssist Technology Device Plugin OpenSSL demo video

The screencast demonstrates the deployment of the [Intel® QAT Device Plugin](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/cmd/qat_plugin/README.md) for
Kubernetes and executes a sample QAT accelerated OpenSSL workload with the OCI
compatible Kata Containers runtime, a lightweight Virtual Machine (VM) that feels
and performs like traditional containers, but provides the workload isolation
and security advantages of VMs. The demo begins with a fully [configured Kubernetes cluster](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/)
and [Kata Containers](https://github.com/kata-containers/documentation/tree/master/install) runtime for workloads.

### Demo platform configuration

- Hardware
    - 1-node, 2x Intel® Xeon® CPU E5-2687W v4 @ 3.00GHz
    - Total memory 251 GB DDR4
    - Intel® QAT C62x chipset
- Software
    - OpenSUSE* 15 (Kernel:4.12.14-lp150.12.22-default)
    - Kubernetes* 1.12
    - Containerd 1.2
    - Kata Containers* 1.3.0
    - Intel® QAT Device Plugin built from main
    - QAT 1.7 L.4.3.0-00033

### Demo steps

1. Load the host drivers and prepare the virtual function (VF) devices.
2. Check the Kubernetes cluster is in good shape.
3. Deploy the Intel® QAT device plugin for Kubernetes.
4. Deploy an Intel® QAT Accelerated OpenSSL workload.
5. Testing!

### Screencast
Intel® QAT Device Plugin deployment

[<img src="https://asciinema.org/a/2N7wF3c9oeCuB9sFqTNm2gmOr.svg" height=600>](https://asciinema.org/a/2N7wF3c9oeCuB9sFqTNm2gmOr)

## Intel® QuickAssist Technology Device Plugin with DPDK demo video

### Demo steps

1. Check health of Kubernetes nodes.
2. Check for allocatable resources.
3. List QAT Virtual Functions.
4. Deploy QAT Device Plugin as a Daemonset.
5. Check again for allocatable resources.
6. List QAT Virtual Functions again, ensuring they are now bound to DPDK driver.
7. View pod specification file for pod requesting QAT VFs.
8. Create pod requesting QAT VFs.
9. Get a shell to the running container and run a DPDK application.

### Screencast
Intel® QAT Device Plugin with DPDK:

[<img src="https://asciinema.org/a/PoWOz4q2lX4AF4K9A2AV1RtSA.svg" width=700>](https://asciinema.org/a/PoWOz4q2lX4AF4K9A2AV1RtSA)