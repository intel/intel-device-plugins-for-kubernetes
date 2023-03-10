# Intel QuickAssist Technology (QAT) device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Modes and Configuration Options](#modes-and-configuration-options)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
    * [Pre-built Images](#pre-built-images)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Demos and Testing](#demos-and-testing)
    * [DPDK QAT Demos](#dpdk-qat-demos)
        * [DPDK Prerequisites](#dpdk-prerequisites)
        * [Deploy the pod](#deploy-the-pod)
        * [Manual test run](#manual-test-run)
        * [Automated test run](#automated-test-run)
    * [OpenSSL QAT Demo](#openssl-qat-demo)
* [Checking for Hardware](#checking-for-hardware)

## Introduction

This Intel QAT device plugin provides support for Intel QAT devices under Kubernetes.
The supported devices are determined by the VF device drivers available in your Linux
Kernel. See the [Prerequisites](#prerequisites) section for more details.

Supported Devices include, but may not be limited to, the following:

- [Intel&reg; Xeon&reg; with Intel&reg; C62X Series Chipset][1]
- Intel&reg; Xeon&reg; with Intel&reg; QAT Gen4 devices
- [Intel&reg; Atom&trade; Processor C3000][2]
- [Intel&reg; Communications Chipset 8925 to 8955 Series][3]

The QAT device plugin provides access to QAT hardware accelerated cryptographic and compression features.
Demonstrations are provided utilising [DPDK](https://doc.dpdk.org/) and [OpenSSL](https://www.openssl.org/).

[Kata Containers](https://katacontainers.io/) QAT integration is documented in the
[Kata Containers documentation repository][6].

## Modes and Configuration Options

The QAT plugin can take a number of command line arguments, summarised in the following table:

| Flag | Argument | Meaning |
|:---- |:-------- |:------- |
| -dpdk-driver | string | DPDK Device driver for configuring the QAT device (default: `vfio-pci`) |
| -kernel-vf-drivers | string | Comma separated VF Device Driver of the QuickAssist Devices in the system. Devices supported: DH895xCC, C62x, C3xxx, 4xxx/401xx/402xx, C4xxx and D15xx (default: `c6xxvf,4xxxvf`) |
| -max-num-devices | int | maximum number of QAT devices to be provided to the QuickAssist device plugin (default: `32`) |
| -mode | string | plugin mode which can be either `dpdk` or `kernel` (default: `dpdk`) |
| -allocation-policy | string | 2 possible values: balanced and packed. Balanced mode spreads allocated QAT VF resources balanced among QAT PF devices, and packed mode packs one QAT PF device full of QAT VF resources before allocating resources from the next QAT PF. (There is no default.) |

The plugin also accepts a number of other arguments related to logging. Please use the `-h` option to see
the complete list of logging related options.

For more details on the `-dpdk-driver` choice, see
[DPDK Linux Driver Guide](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html).

> **Note:**: With Linux 5.9+ kernels the `vfio-pci` module must be loaded with
> `disable_denylist=1` parameter for the QAT device plugin to work correctly with
> devices prior to Gen4 (`4xxx`).

For more details on the available options to the `-kernel-vf-drivers` option, see the list of
vf drivers available in the [Linux Kernel](https://github.com/torvalds/linux/tree/master/drivers/crypto/qat).

If the `-mode` parameter is set to `kernel`, no other parameter documented above are valid,
except the `klog` logging related parameters.
`kernel` mode implements resource allocation based on system configured [logical instances][7].

> **Note**: `kernel` mode is excluded by default from all builds (including those hosted on the Docker hub),
> by default. See the [Build the plugin image](#build-the-plugin-image) section for more details.

The `kernel` mode does not guarantee full device isolation between containers
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

### Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-qat-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_plugin?ref=<RELEASE_VERSION>'
```

Where `<RELEASE_VERSION>` needs to be substituted with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

An alternative kustomization for deploying the plugin is with the debug mode switched on:

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_plugin/overlays/debug?ref=<RELEASE_VERSION>'
```

> **Note**: It is also possible to run the QAT device plugin using a non-root user. To do this,
> the nodes' DAC rules must be configured to allow PCI driver unbinding/binding, device plugin
> socket creation and kubelet registration. Furthermore, the deployments `securityContext` must
> be configured with appropriate `runAsUser/runAsGroup`.

#### Automatic Provisioning

There's a sample [qat initcontainer](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/build/docker/intel-qat-initcontainer.Dockerfile). Regardless of device types, the script running inside the initcontainer enables QAT SR-IOV VFs.

To deploy, run as follows:

```bash
$ kubectl apply -k deployments/qat_plugin/overlays/qat_initcontainer/
```

In addition to the default configuration, you can add device-specific configurations via ConfigMap.

| Device | Possible Configuration | How To Customize | Options | Notes |
|:-------|:-----------------------|:-----------------|:--------|:------|
| 4xxx, 401xx,402xx | [cfg_services](https://github.com/torvalds/linux/blob/42e66b1cc3a070671001f8a1e933a80818a192bf/Documentation/ABI/testing/sysfs-driver-qat) reports the configured services (crypto services or compression services) of the QAT device. | `ServicesEnabled=<value>` | compress:`dc`, crypto:`sym;asym` | Linux 6.0+ kernel is required. |

To create a provisioning config after customizing, run as follows:

```bash
$ kubectl create configmap --namespace=inteldeviceplugins-system qat-config --from-file=deployments/qat_plugin/overlays/qat_initcontainer/qat.conf
```
> **Note**: When deploying the overlay qat_initcontainer, such a manual creation is not necessary since ConfigMap is generated automatically. Just set the values in the config file and deploy the overlay.

When using the operator for deploying the plugin with provisioning config, use `provisioningConfig` field for the name of the ConfigMap, then the config is passed to initcontainer through the volume mount.

There's also a possibility for a node specific congfiguration through passing a nodename via `NODE_NAME` into initcontainer's environment and passing a node specific profile (`qat-$NODE_NAME.conf`) via ConfigMap volume mount.


#### Verify Plugin Registration

Verification of the plugin deployment and detection of QAT hardware can be confirmed by
examining the resource allocations on the nodes:

```bash
$ kubectl describe node <node name> | grep qat.intel.com/generic
 qat.intel.com/generic: 10
 qat.intel.com/generic: 10
```

## Demos and Testing

The below sections cover `DPDK` and `OpenSSL` demos, both of which utilise the
QAT device plugin under Kubernetes.

### DPDK QAT demos

The Data Plane Development Kit (DPDK) QAT demos use DPDK
[crypto-perf](https://doc.dpdk.org/guides/tools/cryptoperf.html) and
[compress-perf](https://doc.dpdk.org/guides/tools/comp_perf.html) utilities to exercise
DPDK QAT Poll-Mode Drivers (PMD). For more information on the tools' parameters, refer to the
website links.

#### DPDK Prerequisites

For the DPDK QAT demos to work, the DPDK drivers must be loaded and configured.
For more information, refer to:
[DPDK Getting Started Guide for Linux](https://doc.dpdk.org/guides/linux_gsg/index.html) and
[DPDK Getting Started Guide, Linux Drivers section](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html)

#### Deploy the pod

In the pod specification file, add container resource request and limit.
For example, `qat.intel.com/generic: <number of devices>` for a container requesting QAT devices.

For a DPDK-based workload, you may need to add hugepage request and limit.

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_dpdk_app/base/
$ kubectl get pods
  NAME                     READY     STATUS    RESTARTS   AGE
  qat-dpdk                 1/1       Running   0          27m
  intel-qat-plugin-5zgvb   1/1       Running   0          3h

```

> **Note**: If the `igb_uio` VF driver is used with the QAT device plugin,
> the workload be deployed with `SYS_ADMIN` capabilities added.

#### Manual Test Run

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

#### Automated Test Run

It is also possible to deploy and run `crypto-perf` using the following
`kustomize` overlays:

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_dpdk_app/test-crypto1
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_dpdk_app/test-compress1
$ kubectl logs qat-dpdk-test-crypto-perf-tc1
$ kubectl logs qat-dpdk-test-compress-perf-tc1
```

> **Note**: for `test-crypto1` and `test-compress1` to work, the cluster must enable
[Kubernetes CPU manager's](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/) `static` policy.

### OpenSSL QAT Demo

Please refer to the [Kata Containers documentation][8] for details on the OpenSSL
QAT acceleration demo.

## Checking for Hardware

In order to utilise the QAT device plugin, QuickAssist SR-IOV virtual functions must be configured.
You can verify this on your nodes by checking for the relevant PCI identifiers:

```bash
for i in 0442 0443 18a1 37c9 6f55 19e3 4941 4943; do lspci -d 8086:$i; done
```

[1]:https://www-ssl.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/purley/intel-xeon-scalable-processors.html
[2]:https://www.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/denverton/ns/atom-processor-c3000-series.html
[3]:https://www.intel.com/content/www/us/en/ethernet-products/gigabit-server-adapters/quickassist-adapter-8950-brief.html
[6]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md
[7]:https://www.intel.com/content/www/us/en/content-details/710060/intel-quickassist-technology-software-for-linux-programmer-s-guide-hw-version-1-7.html
[8]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md#build-openssl-intel-qat-engine-container
[9]:https://www.intel.com/content/www/us/en/content-details/710059/intel-quickassist-technology-software-for-linux-getting-started-guide-hw-version-1-7.html
