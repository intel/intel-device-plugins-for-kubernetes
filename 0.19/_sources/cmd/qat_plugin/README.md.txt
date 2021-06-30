# Intel QuickAssist Technology (QAT) device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
    * [Modes and Configuration options](#modes-and-configuration-options)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
    * [Pre-built image](#pre-built-image)
    * [Getting the source code:](#getting-the-source-code)
    * [Verify node kubelet config](#verify-node-kubelet-config)
    * [Deploying as a DaemonSet](#deploying-as-a-daemonset)
        * [Build the plugin image](#build-the-plugin-image)
        * [Deploy the DaemonSet](#deploy-the-daemonset)
        * [Verify QAT device plugin is registered on master:](#verify-qat-device-plugin-is-registered-on-master)
    * [Deploying by hand](#deploying-by-hand)
        * [Build QAT device plugin](#build-qat-device-plugin)
        * [Deploy QAT plugin](#deploy-qat-plugin)
    * [QAT device plugin Demos](#qat-device-plugin-demos)
        * [DPDK QAT demos](#dpdk-qat-demos)
            * [DPDK Prerequisites](#dpdk-prerequisites)
            * [Build the image](#build-the-image)
            * [Deploy the pod](#deploy-the-pod)
            * [Manual test run](#manual-test-run)
            * [Automated test run](#automated-test-run)
        * [OpenSSL QAT demo](#openssl-qat-demo)
* [Checking for hardware](#checking-for-hardware)

## Introduction

This Intel QAT device plugin provides support for Intel QAT devices under Kubernetes.
The supported devices are determined by the VF device drivers available in your Linux
Kernel. See the [Prerequisites](#prerequisites) section for more details.

Supported Devices include, but may not be limited to, the following:

- [Intel&reg; Xeon&reg; with Intel&reg; C62X Series Chipset][1]
- [Intel&reg; Atom&trade; Processor C3000][2]
- [Intel&reg; Communications Chipset 8925 to 8955 Series][3]

The QAT device plugin provides access to QAT hardware accelerated cryptographic and compression features.
Demonstrations are provided utilising [DPDK](https://doc.dpdk.org/) and [OpenSSL](https://www.openssl.org/).

[Kata Containers](https://katacontainers.io/) QAT integration is documented in the
[Kata Containers documentation repository][6].

### Modes and Configuration options

The QAT plugin can take a number of command line arguments, summarised in the following table:

| Flag | Argument | Meaning |
|:---- |:-------- |:------- |
| -dpdk-driver | string | DPDK Device driver for configuring the QAT device (default: `vfio-pci`) |
| -kernel-vf-drivers | string | Comma separated VF Device Driver of the QuickAssist Devices in the system. Devices supported: DH895xCC,C62x,C3xxx and D15xx (default: `dh895xccvf,c6xxvf,c3xxxvf,d15xxvf`) |
| -max-num-devices | int | maximum number of QAT devices to be provided to the QuickAssist device plugin (default: `32`) |
| -mode | string | plugin mode which can be either `dpdk` or `kernel` (default: `dpdk`) |

The plugin also accepts a number of other arguments related to logging. Please use the `-h` option to see
the complete list of logging related options.

The example [DaemonSet YAML](/deployments/qat_plugin/base/intel-qat-plugin.yaml) passes a number of these
arguments, and takes its default values from the
[QAT default ConfigMap](/deployments/qat_plugin/base/intel-qat-plugin-config.yaml). The following
table summarises the defaults:

| Argument | Variable | Default setting | Explanation |
|:-------- |:-------- |:--------------- |:----------- |
| -dpdk-driver | `$DPDK_DRIVER` | vfio-pci | A more robust and secure choice than the `igb_uio` alternative |
| -kernel-vf-drivers | `$KERNEL_VF_DRIVERS` | dh895xccvf,c6xxvf,c3xxxvf,d15xxvf | Modify to suit your hardware setup |
| -max-num-devices | `$MAX_NUM_DEVICES` | 32 | Modify to suit your hardware setup if necessary |

For more details on the `-dpdk-driver` choice, see
[DPDK Linux Driver Guide](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html).

For more details on the available options to the `-kernel-vf-drivers` option, see the list of
vf drivers available in the [Linux Kernel](https://github.com/torvalds/linux/tree/master/drivers/crypto/qat).

If the `-mode` parameter is set to `kerneldrv`, no other parameter documented above are valid,
except the `klog` logging related parameters.
`kerneldrv` mode implements resource allocation based on system configured [logical instances][7].

> **Note**: `kerneldrv` mode is excluded by default from all builds (including those hosted on the Docker hub),
> by default. See the [Build the plugin image](#build-the-plugin-image) section for more details.

The `kerneldrv` mode does not guarantee full device isolation between containers
and therefore it's not recommended. This mode will be deprecated and removed once `libqat`
implements non-UIO based device access.

## Installation

The below sections cover how to obtain, build and install this component.

The component can be installed either using a DaemonSet or running 'by hand' on each node.

### Prerequisites

The component has the same basic dependancies as the
[generic plugin framework dependencies](../../README.md#about).

You will also need [appropriate hardware installed](#checking-for-hardware).

The QAT plugin requires Linux Kernel VF QAT drivers to be available. These drivers
are available via two methods. One of them must be installed and enabled:

- [Linux Kernel upstream drivers](https://github.com/torvalds/linux/tree/master/drivers/crypto/qat)
- [Intel QuickAssist Technology software for Linux][9]

The demonstrations have their own requirements, listed in their own specific sections.

### Pre-built image

[Pre-built images](https://hub.docker.com/r/intel/intel-qat-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest master branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_plugin?ref=<RELEASE_VERSION>
```

Where `<RELEASE_VERSION>` needs to be substituted with the desired release version, e.g. `v0.18.0`.

An alternative kustomization for deploying the plugin is with the debug mode switched on:

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_plugin/overlays/debug?ref=<RELEASE_VERSION>
```

The deployment YAML files supplied with the component in this repository use the images with the `devel`
tag by default. If you do not build your own local images, your Kubernetes cluster may pull down
the devel images from the Docker hub by default.

To use the release tagged versions of the images, edit the
[YAML deployment files](/deployments/qat_plugin/base/)
appropriately.

### Getting the source code

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Verify node kubelet config

Every node that will be running the plugin must have the
[kubelet device-plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
configured. For each node, check that the kubelet device plugin socket exists:

```bash
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

### Deploying as a DaemonSet

To deploy the plugin as a DaemonSet, you first need to build a container image for the plugin and
ensure that is visible to your nodes. If you do not build your own plugin, your cluster may pull
the image from the pre-built Docker Hub images, depending on your configuration.

#### Build the plugin image

The following will use `docker` to build a local container image called `intel/intel-qat-plugin`
with the tag `devel`. The image build tool can be changed from the default docker by setting the
`BUILDER` argument to the [Makefile](/Makefile).

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make intel-qat-plugin
...
Successfully tagged intel/intel-qat-plugin:devel
```

> **Note**: `kerneldrv` mode is excluded from the build by default. Add `EXTRA_BUILD_ARGS="--build-arg TAGS_KERNELDRV=kerneldrv"` to `make`
> to get `kerneldrv` functionality added to the build.

#### Deploy the DaemonSet

Deploying the plugin involves first the deployment of a
[ConfigMap](/deployments/qat_plugin/base/intel-qat-plugin-config.yaml) and the
[DaemonSet YAML](/deployments/qat_plugin/base/intel-qat-plugin.yaml).

There is a kustomization for deploying both:
```bash
$ kubectl apply -k ${INTEL_DEVICE_PLUGINS_SRC}/deployments/qat_plugin
```
and an alternative kustomization for deploying the plugin in the debug mode:

```bash
$ kubectl apply -k ${INTEL_DEVICE_PLUGINS_SRC}/deployments/qat_plugin/overlays/debug
```

The third option is to deploy the `yaml`s separately:

```bash
$ kubectl create -f ${INTEL_DEVICE_PLUGINS_SRC}/deployments/qat_plugin/base/intel-qat-plugin-config.yaml
$ kubectl create -f ${INTEL_DEVICE_PLUGINS_SRC}/deployments/qat_plugin/base/intel-qat-plugin.yaml
```

> **Note**: It is also possible to run the QAT device plugin using a non-root user. To do this,
> the nodes' DAC rules must be configured to allow PCI driver unbinding/binding, device plugin
> socket creation and kubelet registration. Furthermore, the deployments `securityContext` must
> be configured with appropriate `runAsUser/runAsGroup`.

#### Verify QAT device plugin is registered on master:

Verification of the plugin deployment and detection of QAT hardware can be confirmed by
examining the resource allocations on the nodes:

```bash
$ kubectl describe node <node name> | grep qat.intel.com/generic
 qat.intel.com/generic: 10
 qat.intel.com/generic: 10
```

### Deploying by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

#### Build QAT device plugin

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make qat_plugin
```

#### Deploy QAT plugin

Deploy the plugin on a node by running it as `root`. The below is just an example - modify the
paramaters as necessary for your setup:

```bash
$ sudo -E ${INTEL_DEVICE_PLUGINS_SRC}/cmd/qat_plugin/qat_plugin \
-dpdk-driver igb_uio -kernel-vf-drivers dh895xccvf -max-num-devices 10 -debug
QAT device plugin started
Discovered Devices below:
03:01.0 device: corresponding DPDK device detected is uio0
03:01.1 device: corresponding DPDK device detected is uio1
03:01.2 device: corresponding DPDK device detected is uio2
03:01.3 device: corresponding DPDK device detected is uio3
03:01.4 device: corresponding DPDK device detected is uio4
03:01.5 device: corresponding DPDK device detected is uio5
03:01.6 device: corresponding DPDK device detected is uio6
03:01.7 device: corresponding DPDK device detected is uio7
03:02.0 device: corresponding DPDK device detected is uio8
03:02.1 device: corresponding DPDK device detected is uio9
The number of devices discovered are:10
device-plugin start server at: /var/lib/kubelet/device-plugins/intelQAT.sock
device-plugin registered
ListAndWatch: Sending device response
```

### QAT device plugin Demos

The below sections cover `DPDK` and `OpenSSL` demos, both of which utilise the
QAT device plugin under Kubernetes.

#### DPDK QAT demos

The Data Plane Development Kit (DPDK) QAT demos use DPDK
[crypto-perf](https://doc.dpdk.org/guides/tools/cryptoperf.html) and
[compress-perf](https://doc.dpdk.org/guides/tools/comp_perf.html) utilities to exercise
DPDK QAT Poll-Mode Drivers (PMD). For more information on the tools' parameters, refer to the
website links.

##### DPDK Prerequisites

For the DPDK QAT demos to work, the DPDK drivers must be loaded and configured.
For more information, refer to:
[DPDK Getting Started Guide for Linux](https://doc.dpdk.org/guides/linux_gsg/index.html) and
[DPDK Getting Started Guide, Linux Drivers section](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html)

##### Build the image

The demo uses a container image. You can either use the
[pre-built image from the Docker Hub](https://hub.docker.com/r/intel/crypto-perf), or build your own local copy.

To build the DPDK demo image:

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ ./build-image.sh crypto-perf
...
Successfully tagged crypto-perf:devel
```

##### Deploy the pod

In the pod specification file, add container resource request and limit.
For example, `qat.intel.com/generic: <number of devices>` for a container requesting QAT devices.

For a DPDK-based workload, you may need to add hugepage request and limit.

```bash
$ kubectl apply -k ${INTEL_DEVICE_PLUGINS_SRC}/deployments/qat_dpdk_app/base/
$ kubectl get pods
  NAME                     READY     STATUS    RESTARTS   AGE
  qat-dpdk                 1/1       Running   0          27m
  intel-qat-plugin-5zgvb   1/1       Running   0          3h

```

> **Note**: The deployment example above uses [kustomize](https://github.com/kubernetes-sigs/kustomize)
> that is available in kubectl since Kubernetes v1.14 release.

##### Manual test run

Manually execute the `dpdk-test-crypto-perf` application to review the logs:

```bash
$ kubectl exec -it qat-dpdk bash

$ dpdk-test-crypto-perf -l 6-7 -w $QAT1 \
-d /usr/lib64/librte_mempool_ring.so.1.1 \
-d /usr/lib64/librte_pmd_qat.so.1.1 -- \
--ptest throughput --devtype crypto_qat \
--optype cipher-only --cipher-algo aes-cbc --cipher-op encrypt \
--cipher-key-sz 16 --total-ops 10000000 --burst-sz 32 --buffer-sz 64
```

> **Note**: Adapt the `.so` versions to what the DPDK version in the container provides.

##### Automated test run

It is also possible to deploy and run `crypto-perf` using the following
`kustomize` overlays:

```bash
$ kubectl apply -k ${INTEL_DEVICE_PLUGINS_SRC}/deployments/qat_dpdk_app/test-crypto1
$ kubectl apply -k ${INTEL_DEVICE_PLUGINS_SRC}/deployments/qat_dpdk_app/test-compress1
$ kubectl logs qat-dpdk-test-crypto-perf-tc1
$ kubectl logs qat-dpdk-test-compress-perf-tc1
```

> **Note**: for `test-crypto1` and `test-compress1` to work, the cluster must enable
[Kubernetes CPU manager's](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/) `static` policy.

#### OpenSSL QAT demo

Please refer to the [Kata Containers documentation][8] for details on the OpenSSL
QAT acceleration demo.

## Checking for hardware

In order to utilise the QAT device plugin, QuickAssist SR-IOV virtual functions must be configured.
You can verify this on your nodes by checking for the relevant PCI identifiers:

```bash
for i in 0442 0443 37c9 19e3; do lspci -d 8086:$i; done
```

[1]:https://www-ssl.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/purley/intel-xeon-scalable-processors.html
[2]:https://www.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/denverton/ns/atom-processor-c3000-series.html
[3]:https://www.intel.com/content/www/us/en/ethernet-products/gigabit-server-adapters/quickassist-adapter-8950-brief.html
[6]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md
[7]:https://01.org/sites/default/files/downloads//336210-009qatswprogrammersguide.pdfhttps://01.org/sites/default/files/downloads//336210-009qatswprogrammersguide.pdf
[8]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md#build-openssl-intel-qat-engine-container
[9]:https://01.org/sites/default/files/downloads/intelr-quickassist-technology/336212qatswgettingstartedguiderev003.pdf