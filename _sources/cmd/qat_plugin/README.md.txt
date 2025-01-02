# Intel QuickAssist Technology (QAT) device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Modes and Configuration Options](#modes-and-configuration-options)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
    * [Pre-built Images](#pre-built-images)
    * [Automatic Provisioning](#automatic-provisioning)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Demos and Testing](#demos-and-testing)
    * [DPDK QAT Demos](#dpdk-qat-demos)
        * [DPDK Prerequisites](#dpdk-prerequisites)
        * [Deploy the pod](#deploy-the-pod)
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

The QAT device plugin provides access to QAT hardware accelerated cryptographic and compression features
through the SR-IOV virtual functions (VF). Demonstrations are provided utilising [DPDK](https://doc.dpdk.org/) and [OpenSSL](https://www.openssl.org/).

QAT Kubernetes resources show up as `qat.intel.com/generic` on systems _before_ QAT Gen4 (4th Gen Xeon&reg;) and `qat.intel.com/[cy|dc]` on QAT Gen4.

## Modes and Configuration Options

The QAT plugin can take a number of command line arguments, summarised in the following table:

| Flag | Argument | Meaning |
|:---- |:-------- |:------- |
| -dpdk-driver | string | DPDK Device driver for configuring the QAT device (default: `vfio-pci`) |
| -kernel-vf-drivers | string | Comma separated list of the QuickAssist VFs to search and use in the system. Devices supported: DH895xCC, C62x, C3xxx, 4xxx/401xx/402xx, 420xx, C4xxx and D15xx (default: `4xxxvf,420xxvf`) |
| -max-num-devices | int | maximum number of QAT devices to be provided to the QuickAssist device plugin (default: `64`) |
| -mode | string | Deprecated: plugin mode which can be either `dpdk` or `kernel` (default: `dpdk`).|
| -allocation-policy | string | 2 possible values: balanced and packed. Balanced mode spreads allocated QAT VF resources balanced among QAT PF devices, and packed mode packs one QAT PF device full of QAT VF resources before allocating resources from the next QAT PF. (There is no default.) |

The plugin also accepts a number of other arguments related to logging. Please use the `-h` option to see
the complete list of logging related options.

For more details on the `-dpdk-driver` choice, see
[DPDK Linux Driver Guide](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html).

> **Note:**: With Linux 5.9+ kernels the `vfio-pci` module must be loaded with
> `disable_denylist=1` parameter for the QAT device plugin to work correctly with
> devices prior to Gen4 (`4xxx`).

For more details on the available options to the `-kernel-vf-drivers` option, see the list of
vf drivers available in the [Linux Kernel](https://github.com/torvalds/linux/tree/master/drivers/crypto/intel/qat).

If the `-mode` parameter is set to `kernel`, no other parameter documented above are valid,
except the `klog` logging related parameters.
`kernel` mode implements resource allocation based on system configured [logical instances][7] and
it does not guarantee full device isolation between containers. Therefore, it's not recommended.

> **Note**: `-mode` parameter is deprecated and it is also not made available as an option to
> the operator based deployment. Furthermore, `kernel` mode is excluded by default from all builds (including those hosted on the Docker hub),
> by default. See the [Build the plugin image](#build-the-plugin-image) section for more details.

## Installation

The below sections cover how to obtain, build and install this component.

The component can be installed either using a DaemonSet or running 'by hand' on each node.

### Prerequisites

The component has the same basic dependencies as the
[generic plugin framework dependencies](../../README.md#about).

You will also need [appropriate hardware installed](#checking-for-hardware).

The QAT plugin requires Linux Kernel VF QAT drivers to be available. These drivers
are available via two methods. One of them must be installed and enabled:

- [Linux Kernel upstream drivers](https://github.com/torvalds/linux/tree/master/drivers/crypto/intel/qat)
- [Intel QuickAssist Technology software for Linux][9] (**Note**: not applicable to QAT Gen4)

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

### Automatic Provisioning

There's a sample [qat initcontainer](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/build/docker/intel-qat-initcontainer.Dockerfile). Regardless of device types, the script running inside the initcontainer enables QAT SR-IOV VFs.

To deploy, run as follows:

```bash
$ kubectl apply -k deployments/qat_plugin/overlays/qat_initcontainer/
```

In addition to the default configuration, you can add device-specific configurations via ConfigMap.

| Device | Possible Configuration | How To Customize | Options | Notes |
|:-------|:-----------------------|:-----------------|:--------|:------|
| 4xxx, 401xx, 402xx, 420xx | [cfg_services](https://github.com/torvalds/linux/blob/v6.6-rc5/Documentation/ABI/testing/sysfs-driver-qat) reports the configured services (crypto services or compression services) of the QAT device. | `ServicesEnabled=<value>` | compress:`dc`, crypto:`sym;asym`, <br>crypto+compress:`asym;dc`,<br>crypto+compress:`sym;dc` | 4xxx/401xx/402xx: Linux 6.0+ kernel. 420xx: Linux 6.8+ kernel. |
| 4xxx, 401xx, 402xx, 420xx | [auto_reset](https://github.com/torvalds/linux/blob/a38297e3fb012ddfa7ce0321a7e5a8daeb1872b6/Documentation/ABI/testing/sysfs-driver-qat#L145) reports the setting of the QAT device's automatic error recovery functionality. | `AutoresetEnabled=<value>` | `on`, `off`, | Linux 6.8+ kernel. |

To create a provisioning `configMap`, run the following command before deploying initcontainer:

```bash
$ kubectl create configmap --namespace=inteldeviceplugins-system qat-config --from-file=/path/to/qat.conf
```
or
```bash
$ kubectl create configmap --namespace=inteldeviceplugins-system --from-literal "qat.conf=ServicesEnabled=<option>" qat-config
```

When using the operator for deploying the plugin with provisioning config, use `provisioningConfig` field for the name of the ConfigMap, then the config is passed to initcontainer through the volume mount.

There's also a possibility for a node specific congfiguration through passing a nodename via `NODE_NAME` into initcontainer's environment and passing a node specific profile (`qat-$NODE_NAME.conf`) via ConfigMap volume mount.

Existing DaemonSet annotations can be updated through CR annotations in [deviceplugin_v1_qatdeviceplugin.yaml](../../deployments/operator/samples/deviceplugin_v1_qatdeviceplugin.yaml).

By default, the operator based deployment sets AppArmor policy to `"unconfined"` but this can be overridden by setting the AppArmor annotation to a new value in the CR annotations.

For non-operator plugin deployments such annotations can be dropped with the kustomization if required.

### Verify Plugin Registration

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

- For the DPDK QAT demos to work, the DPDK drivers must be loaded and configured.
For more information, refer to:
[DPDK Getting Started Guide for Linux](https://doc.dpdk.org/guides/linux_gsg/index.html) and
[DPDK Getting Started Guide, Linux Drivers section](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html)

- You may need to add hugepage request and limit.

- The cluster must enable [Kubernetes CPU manager's](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/) `static` policy.

> **Note**: If the `igb_uio` VF driver is used with the QAT device plugin,
> the workload be deployed with `SYS_ADMIN` capabilities added.

#### Deploy the pod

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_dpdk_app/
$ kubectl get pods
  NAME                          READY     STATUS    RESTARTS   AGE
  qat-dpdk-test-crypto-perf     1/1       Running   0          27m
  qat-dpdk-test-compress-perf   1/1       Running   0          27m
  intel-qat-plugin-5zgvb        1/1       Running   0          3h
```

Both pods for [crypto-perf](../..//deployments/qat_dpdk_app/crypto-perf) and [compress-perf](../../deployments/qat_dpdk_app/compress-perf) are generated by default using `kustomize`.

It is also possible to deploy and run `crypto-perf` or `compress-perf` only.

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_dpdk_app/crypto-perf
$ kubectl logs qat-dpdk-test-crypto-perf
```

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/qat_dpdk_app/compress-perf
$ kubectl logs qat-dpdk-test-compress-perf
```

### OpenSSL QAT Demo

Please refer to the [Kata Containers documentation][8] for details on the OpenSSL
QAT acceleration demo.

## Checking for Hardware

In order to utilise the QAT device plugin, QuickAssist SR-IOV virtual functions must be configured.
You can verify this on your nodes by checking for the relevant PCI identifiers:

```bash
for i in 0442 0443 18a1 37c9 6f55 19e3 4941 4943 4945 4947; do lspci -d 8086:$i; done
```

[1]:https://www-ssl.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/purley/intel-xeon-scalable-processors.html
[2]:https://www.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/denverton/ns/atom-processor-c3000-series.html
[3]:https://www.intel.com/content/www/us/en/ethernet-products/gigabit-server-adapters/quickassist-adapter-8950-brief.html
[6]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md
[7]:https://www.intel.com/content/www/us/en/content-details/710060/intel-quickassist-technology-software-for-linux-programmer-s-guide-hw-version-1-7.html
[8]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md#build-openssl-intel-qat-engine-container
[9]:https://www.intel.com/content/www/us/en/content-details/710059/intel-quickassist-technology-software-for-linux-getting-started-guide-hw-version-1-7.html
