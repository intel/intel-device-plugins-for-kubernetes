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

The QAT device plugin provides access to QAT hardware accelerated cryptographic and compression features
through the SR-IOV virtual functions (VF). Demonstrations are provided utilising [DPDK](https://doc.dpdk.org/) and [OpenSSL](https://www.openssl.org/).

QAT Kubernetes resources show up as `qat.intel.com/generic` on systems _before_ QAT Gen4 (4th Gen Xeon&reg;) and `qat.intel.com/[cy|dc]` on QAT Gen4.

## Modes and Configuration Options

The QAT plugin can take a number of command line arguments, summarised in the following table:

| Flag | Argument | Meaning |
|:---- |:-------- |:------- |
| -dpdk-driver | string | DPDK Device driver for configuring the QAT device (default: `vfio-pci`) |
| -kernel-vf-drivers | string | Comma separated list of the QuickAssist VFs to search and use in the system. Devices supported: DH895xCC, C62x, C3xxx, 4xxx/401xx/402xx, C4xxx and D15xx (default: `c6xxvf,4xxxvf`) |
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

The component has the same basic dependancies as the
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
| 4xxx, 401xx,402xx | [cfg_services](https://github.com/torvalds/linux/blob/v6.6-rc5/Documentation/ABI/testing/sysfs-driver-qat) reports the configured services (crypto services or compression services) of the QAT device. | `ServicesEnabled=<value>` | compress:`dc`, crypto:`sym;asym`, <br>crypto+compress:`asym;dc`,<br>crypto+compress:`sym;dc` | Linux 6.0+ kernel is required. |

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

For the DPDK QAT demos to work, the DPDK drivers must be loaded and configured.
For more information, refer to:
[DPDK Getting Started Guide for Linux](https://doc.dpdk.org/guides/linux_gsg/index.html) and
[DPDK Getting Started Guide, Linux Drivers section](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html)

#### Deploy the pod

In the pod specification file, add container resource request and limit.
For example, `qat.intel.com/generic: <number of devices>` for a container requesting QAT devices.

For a DPDK-based workload, you may need to add hugepage request and limit.

##### NOTES
1. In the below file, modify line 21 and 26 to reflect the custom resources in your worker node. cy=crypto and dc=compression
It should look like below based on your scenario

qat.intel.com/cy: '4'
or
qat.intel.com/dc: '4'
https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/deployments/qat_dpdk_app/base/crypto-perf-dpdk-pod-requesting-qat.yaml

Also, you can check by describing the workr node and verify the Allocatable resources.
![Alt text](worker-node-describe.png)


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

##### revised command
```bash
$ kubectl exec -it qat-dpdk bash

$ dpdk-test-crypto-perf --no-telemetry -l 4,112,116 -a $QAT1 -- \
--ptest throughput --devtype crypto_qat --optype cipher-only \
--cipher-algo aes-cbc --cipher-op encrypt --cipher-key-sz 16 \
--total-ops 10000000 --burst-sz 32 --buffer-sz 64
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

## NOTES
1. The initcontainer deployment will split the qat resources into two: cy and dc.
2. cy stands for crypto and dc for compression. Depending on what your workload needs, you can then request the either for the container.


[1]:https://www-ssl.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/purley/intel-xeon-scalable-processors.html
[2]:https://www.intel.com/content/www/us/en/design/products-and-solutions/processors-and-chipsets/denverton/ns/atom-processor-c3000-series.html
[3]:https://www.intel.com/content/www/us/en/ethernet-products/gigabit-server-adapters/quickassist-adapter-8950-brief.html
[6]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md
[7]:https://www.intel.com/content/www/us/en/content-details/710060/intel-quickassist-technology-software-for-linux-programmer-s-guide-hw-version-1-7.html
[8]:https://github.com/kata-containers/documentation/blob/master/use-cases/using-Intel-QAT-and-kata.md#build-openssl-intel-qat-engine-container
[9]:https://www.intel.com/content/www/us/en/content-details/710059/intel-quickassist-technology-software-for-linux-getting-started-guide-hw-version-1-7.html


# ERRORS and Troubleshooting:

# Error-1  
logs -n default           intel-qat-plugin-fjqnw
(base) [root@spr-bkc-pc-3 ~]# k logs -n default           intel-qat-plugin-fjqnw

Device scan failed: open /sys/bus/pci/drivers/vfio-pci/new_id: no such file or directory
write to driver failed: 8086 4941
github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/dpdkdrv.writeToDriver
        /go/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/dpdkdrv/dpdkdrv.go:444
github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/dpdkdrv.(*DevicePlugin).setupDeviceIDs
        /go/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/dpdkdrv/dpdkdrv.go:195
github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/dpdkdrv.(*DevicePlugin).Scan
        /go/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/dpdkdrv/dpdkdrv.go:209
github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin.(*Manager).Run.func1
        /go/src/github.com/intel/intel-device-plugins-for-kubernetes/pkg/deviceplugin/manager.go:100
runtime.goexit
        /usr/local/go/src/runtime/asm_amd64.s:1594
failed to set device ID 4941 for vfio-pci. Driver module not loaded?

## Solution-1
modprobe vfio-pci

## Error-2 - invalid options dpdk-test-crypto-perf
root@qat-dpdk:/# dpdk-test-crypto-perf -l 6-7 -w $QAT1 \
-d /usr/lib64/librte_mempool_ring.so.1.1 \
-d /usr/lib64/librte_pmd_qat.so.1.1 -- \
--ptest throughput --devtype crypto_qat \
--optype cipher-only --cipher-algo aes-cbc --cipher-op encrypt \
--cipher-key-sz 16 --total-ops 10000000 --burst-sz 32 --buffer-sz 64
dpdk-test-crypto-perf: invalid option -- 'w'
EAL: Detected CPU lcores: 128
EAL: Detected NUMA nodes: 2
dpdk-test-crypto-perf: invalid option -- 'w'

Usage: dpdk-test-crypto-perf [options]

EAL common options:
  -c COREMASK         Hexadecimal bitmask of cores to run on
  -l CORELIST         List of cores to run on
                      The argument format is <c1>[-c2][,c3[-c4],...]
                      where c1, c2, etc are core indexes between 0 and 128
  --lcores COREMAP    Map lcore set to physical cpu set
                      The argument format is
                            '<lcores[@cpus]>[<,lcores[@cpus]>...]'
                      lcores and cpus list are grouped by '(' and ')'
                      Within the group, '-' is used for range separator,
                      ',' is used for single number separator.
                      '( )' can be omitted for single element group,
                      '@' can be omitted if cpus and lcores have the same value
  -s SERVICE COREMASK Hexadecimal bitmask of cores to be used as service cores
  --main-lcore ID     Core ID that is used as main
  --mbuf-pool-ops-name Pool ops name for mbuf to use
  -n CHANNELS         Number of memory channels
  -m MB               Memory to allocate (see also --socket-mem)
  -r RANKS            Force number of memory ranks (don't detect)
  -b, --block         Add a device to the blocked list.
                      Prevent EAL from using this device. The argument
                      format for PCI devices is <domain:bus:devid.func>.
  -a, --allow         Add a device to the allow list.
                      Only use the specified devices. The argument format
                      for PCI devices is <[domain:]bus:devid.func>.
                      This option can be present several times.
                      [NOTE: allow cannot be used with block option]
  --vdev              Add a virtual device.
                      The argument format is <driver><id>[,key=val,...]
                      (ex: --vdev=net_pcap0,iface=eth2).
  --iova-mode   Set IOVA mode. 'pa' for IOVA_PA
                      'va' for IOVA_VA
  -d LIB.so|DIR       Add a driver or driver directory
                      (can be used multiple times)
  --vmware-tsc-map    Use VMware TSC map instead of native RDTSC
  --proc-type         Type of this process (primary|secondary|auto)
  --syslog            Set syslog facility
  --log-level=<level> Set global log level
  --log-level=<type-match>:<level>
                      Set specific log level
  --log-level=help    Show log types and levels
  --trace=<regex-match>
                      Enable trace based on regular expression trace name.
                      By default, the trace is disabled.
                      User must specify this option to enable trace.
  --trace-dir=<directory path>
                      Specify trace directory for trace output.
                      By default, trace output will created at
                      $HOME directory and parameter must be
                      specified once only.
  --trace-bufsz=<int>
                      Specify maximum size of allocated memory
                      for trace output for each thread. Valid
                      unit can be either 'B|K|M' for 'Bytes',
                      'KBytes' and 'MBytes' respectively.
                      Default is 1MB and parameter must be
                      specified once only.
  --trace-mode=<o[verwrite] | d[iscard]>
                      Specify the mode of update of trace
                      output file. Either update on a file can
                      be wrapped or discarded when file size
                      reaches its maximum limit.
                      Default mode is 'overwrite' and parameter
                      must be specified once only.
  -v                  Display version information on startup
  -h, --help          This help
  --in-memory   Operate entirely in memory. This will
                      disable secondary process support
  --base-virtaddr     Base virtual address
  --telemetry   Enable telemetry support (on by default)
  --no-telemetry   Disable telemetry support
  --force-max-simd-bitwidth Force the max SIMD bitwidth

EAL options for DEBUG use only:
  --huge-unlink[=existing|always|never]
                      When to unlink files in hugetlbfs
                      ('existing' by default, no value means 'always')
  --no-huge           Use malloc instead of hugetlbfs
  --no-pci            Disable PCI
  --no-hpet           Disable HPET
  --no-shconf         No shared config (mmap'd files)

EAL Linux options:
  --socket-mem        Memory to allocate on sockets (comma separated values)
  --socket-limit      Limit memory allocation on sockets (comma separated values)
  --huge-dir          Directory where hugetlbfs is mounted
  --file-prefix       Prefix for hugepage filenames
  --create-uio-dev    Create /dev/uioX (usually done by hotplug)
  --vfio-intr         Interrupt mode for VFIO (legacy|msi|msix)
  --vfio-vf-token     VF token (UUID) shared between SR-IOV PF and VFs
  --legacy-mem        Legacy memory mode (no dynamic allocation, contiguous segments)
  --single-file-segments Put all hugepage memory in single files
  --match-allocations Free hugepages exactly as allocated
  --huge-worker-stack[=size]
                      Allocate worker thread stacks from hugepage memory.
                      Size is in units of kbytes and defaults to system
                      thread stack size if not specified.

EAL: FATAL: Invalid 'command line' arguments.
EAL: Invalid 'command line' arguments.
EAL: Error - exiting with code: 1
  Cause: Invalid EAL arguments!

## Soltuion-2
Command below has the correct arguments

dpdk-test-crypto-perf --no-telemetry -l 4,112,116 -a $QAT1 -- --ptest throughput --devtype crypto_qat --optype cipher-only --cipher-algo aes-cbc --cipher-op encrypt --cipher-key-sz 16 --total-ops 10000000 --burst-sz 32 --buffer-sz 64


# Error-3
 boost headers required 
 $ ./configure --enable-icp-sriov=host --enable-icp-debug
.
.
.
.
configure: error:

    boost headers required.


# Solution-3 
yum install boost-devel
 1062  dnf install kernel-spr-bkc-pc-devel-matched kernel-spr-bkc-pc-devel --skip-broken
 1064  yum -y install systemd-devel
 1066  dnf --enablerepo=powertools install yasm
 1067  ./configure --enable-icp-sriov=host --enable-icp-debug

# Error-4
unable to find kernel headers while running make

# Solution-4
yum install -y kernel-headers kernel-devel elfutils-libelf-devel



