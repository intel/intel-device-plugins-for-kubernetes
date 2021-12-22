# Intel DLB device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Deploy with pre-built container image](#deploy-with-pre-built-container-image)
    * [Getting the source code](#getting-the-source-code)
    * [Deploying as a DaemonSet](#deploying-as-a-daemonset)
        * [Build the plugin image](#build-the-plugin-image)
        * [Deploy plugin DaemonSet](#deploy-plugin-daemonset)
    * [Deploy by hand](#deploy-by-hand)
        * [Build the plugin](#build-the-plugin)
        * [Run the plugin as administrator](#run-the-plugin-as-administrator)
    * [Verify plugin registration](#verify-plugin-registration)
    * [Testing the plugin](#testing-the-plugin)

## Introduction

This Intel DLB device plugin provides support for [Intel DLB](https://builders.intel.com/docs/networkbuilders/SKU-343247-001US-queue-management-and-load-balancing-on-intel-architecture.pdf) devices under Kubernetes.

### DLB2 driver configuration for PFs
You should first build and load the dlb2 driver. Get [DLB software release](https://downloadmirror.intel.com/690271/dlblinuxsrcrelease7.4.020211006.txz), build the kernel module and load the module as follows:
```bash
$ wget -q https://downloadmirror.intel.com/690271/dlblinuxsrcrelease7.4.020211006.txz -O- | tar -Jx
$ cd /dlb/driver/dlb2/ && make
$ sudo insmod dlb2.ko
```

Then, available dlb device nodes are visible in devfs.
```bash
$ ls -1 /dev/dlb*
/dev/dlb0  /dev/dlb1  /dev/dlb2 ...
```

### VF configuration using a DPDK tool (but with dlb2 driver)
If you configure SR-IOV/VF (virtual functions), continue the following configurations. This instruction uses DPDK tool to check eventdev devices, unbind a VF device, and bind dlb2 driver to a VF device.

Patch dpdk sources to work with DLB:
```bash
$ wget -q https://fast.dpdk.org/rel/dpdk-20.11.3.tar.xz -O- | tar -Jx
$ wget -q https://downloadmirror.intel.com/690271/dlblinuxsrcrelease7.4.020211006.txz -O- | tar -Jx
$ cd ./dpdk-stable-20.11.3 && patch -p1 < ../dlb/dpdk/dpdk_dlb_v20.11.3_29751a4_diff.patch
$ sed -i 's/270b,2710,2714/270b,2710,2711,2714/g' ./usertools/dpdk-devbind.py
```

List eventdev devices:
```bash
$ ./usertools/dpdk-devbind.py -s | grep -A10 ^Eventdev
Eventdev devices using kernel driver
====================================
0000:6d:00.0 'Device 2710' drv=dlb2 unused= 
0000:72:00.0 'Device 2710' drv=dlb2 unused= 
...
```

Enable virtual functions:
```bash
$ echo 4 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/sriov_numvfs
```
> **Note:**: If it fails saying "No such file or directory," it may be bound to vfio-pci driver. Bind the device to dlb2 driver.

Check if new dlb device nodes appear:
```bash
$ ls -1 /dev/dlb*
/dev/dlb0  /dev/dlb1  /dev/dlb10 /dev/dlb11 ... /dev/dlb8  /dev/dlb9
```

Check that new eventdev devices appear:
```bash
$ ./usertools/dpdk-devbind.py -s | grep -A14 ^Eventdev
Eventdev devices using kernel driver
====================================
0000:6d:00.0 'Device 2710' drv=dlb2 unused= 
0000:6d:00.1 'Device 2711' drv=dlb2 unused= 
0000:6d:00.2 'Device 2711' drv=dlb2 unused= 
0000:6d:00.3 'Device 2711' drv=dlb2 unused= 
0000:6d:00.4 'Device 2711' drv=dlb2 unused= 
0000:72:00.0 'Device 2710' drv=dlb2 unused= 
...
```

Assign PF resources to VF:
> **Note:**: The process below is only for the first vf resource among 4 resources. Repeat for other vfN_resources in /sys/bus/pci/devices/0000\:6d\:00.0/, and then bind dlb2 driver to 0000:6d:00.M that corresponds to vfN_resources.

- Unbind driver from the VF device before configuring it.
```bash
$ sudo ./usertools/dpdk-devbind.py --unbind 0000:6d:00.1
```

- Assign PF resources to VF:
```bash
$ echo 2048 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_atomic_inflights &&
  echo 2048 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_dir_credits &&
  echo 64 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_dir_ports &&
  echo 2048 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_hist_list_entries &&
  echo 8192 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_ldb_credits &&
  echo 64 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_ldb_ports &&
  echo 32 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_ldb_queues &&
  echo 32 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_sched_domains &&
  echo 2 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_sn0_slots &&
  echo 2 | sudo tee -a /sys/bus/pci/devices/0000\:6d\:00.0/vf0_resources/num_sn1_slots
```

- Bind driver back to the VF device:
```bash
$ sudo ./usertools/dpdk-devbind.py --bind dlb2 0000:6d:00.1
```


### Verification of well-configured devices:
Run libdlb example app:
> **Note:**: Alternative way is to use this [Dockerfile](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/dlb-libdlb-demo/Dockerfile) for running tests.

```bash
$ ls
dlb dpdk-stable-20.11.3
$ cd ./dlb/libdlb/ && make && sudo LD_LIBRARY_PATH=$PWD ./examples/dir_traffic -n 128 -d 1
# For running test for /dev/dlbN, replace 1 with N.
```

Run dpdk example app:
> **Note:**: Alternative way is to use this [Dockerfile](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/dlb-dpdk-demo/Dockerfile) for patching and building DPDK and running tests.

- Install build dependencies and build dpdk:
```bash
$ sudo apt-get update && sudo apt-get install build-essential meson python3-pyelftools libnuma-dev python3-pip && sudo pip install ninja
# This configuration is based on Ubuntu/Debian distribution. For other distributions that do not use apt, install the dependencies using another way.
$ ls
dlb dpdk-stable-20.11.3
$ cd ./dpdk-stable-20.11.3 && meson setup --prefix $(pwd)/installdir builddir && ninja -C builddir install
```

- Run eventdev test
```bash
sudo ./builddir/app/dpdk-test-eventdev --no-huge --vdev='dlb2_event,dev_id=1' -- --test=order_queue --nb_flows 64 --nb_pkts 512 --plcores 1 --wlcores 2-7
# For running test for /dev/dlbN, replace 1 with N.
```

## Installation

The following sections detail how to obtain, build, deploy and test the DLB device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Deploy with pre-built container image

[Pre-built images](https://hub.docker.com/r/intel/intel-dlb-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/dlb_plugin?ref=<REF>
daemonset.apps/intel-dlb-plugin created
```

Where `<REF>` needs to be substituted with the desired git ref, e.g. `main`.

Nothing else is needed. But if you want to deploy a customized version of the plugin read further.

### Getting the source code

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Deploying as a DaemonSet

To deploy the dlb plugin as a daemonset, you first need to build a container image for the
plugin and ensure that is visible to your nodes.

#### Build the plugin image

The following will use `docker` to build a local container image called
`intel/intel-dlb-plugin` with the tag `devel`.

The image build tool can be changed from the default `docker` by setting the `BUILDER` argument
to the [`Makefile`](Makefile).

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make intel-dlb-plugin
...
Successfully tagged intel/intel-dlb-plugin:devel
```

#### Deploy plugin DaemonSet

You can then use the [example DaemonSet YAML](/deployments/dlb_plugin/base/intel-dlb-plugin.yaml)
file provided to deploy the plugin. The default kustomization that deploys the YAML as is:

```bash
$ kubectl apply -k deployments/dlb_plugin
daemonset.apps/intel-dlb-plugin created
```

### Deploy by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

#### Build the plugin

First we build the plugin:

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make dlb_plugin
```

#### Run the plugin as administrator

Now we can run the plugin directly on the node:

```bash
$ sudo -E ${INTEL_DEVICE_PLUGINS_SRC}/cmd/dlb_plugin/dlb_plugin
```

### Verify plugin registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{range $k,$v:=.status.allocatable}}{{"  "}}{{$k}}{{": "}}{{$v}}{{"\n"}}{{end}}{{end}}' | grep '^\([^ ]\)\|\(  dlb\)'
master
  dlb.intel.com/pf: 7
  dlb.intel.com/vf: 4
```

### Testing the plugin

We can test the plugin is working by deploying the provided example test images (dlb-libdlb-demo and dlb-dpdk-demo).

1. Create a pod running unit tests off the local Docker image:

    ```bash
    $ kubectl apply -f ${INTEL_DEVICE_PLUGINS_SRC}/demo/dlb-libdlb-demo-pod.yaml
    pod/dlb-libdlb-demo-pod created
    ```

    ```bash
    $ kubectl apply -f ${INTEL_DEVICE_PLUGINS_SRC}/demo/dlb-dpdk-demo-pod.yaml
    pod/dlb-dpdk-demo-pod created
    ```

1. Wait until pod is completed:

    ```bash
    $ kubectl get pods | grep dlb-.*-demo
    NAME              READY   STATUS      RESTARTS   AGE
    dlb-dpdk-demo     0/2     Completed   0          79m
    dlb-libdlb-demo   0/2     Completed   0          18h
    ```

1. Review the job's logs:

    ```bash
    $ kubectl logs dlb-libdlb-demo <pf/vf>
    <log output>
    ```

    ```bash
    $ kubectl logs dlb-dpdk-demo <pf/vf>
    <log output>
    ```

    If the pod did not successfully launch, possibly because it could not obtain the DLB
    resource, it will be stuck in the `Pending` status:

    ```bash
    $ kubectl get pods
    NAME              READY   STATUS      RESTARTS   AGE
    dlb-dpdk-demo     0/2     Pending     0          3s
    dlb-libdlb-demo   0/2     Pending     0          10s
    ```

    This can be verified by checking the Events of the pod:

    ```bash
    $ kubectl describe pod dlb-libdlb-demo | grep -A3 Events:
    Events:
    Type     Reason            Age   From               Message
    ----     ------            ----  ----               -------
    Warning  FailedScheduling  85s   default-scheduler  0/1 nodes are available: 1 Insufficient dlb.intel.com/pf, 1 Insufficient dlb.intel.com/vf.
    ```
