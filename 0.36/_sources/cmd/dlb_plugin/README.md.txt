# Intel DLB device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Pre-built Images](#pre-built-images)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Testing and Demos](#testing-and-demos)

## Introduction

This Intel DLB device plugin provides support for [Intel DLB](https://builders.intel.com/docs/networkbuilders/SKU-343247-001US-queue-management-and-load-balancing-on-intel-architecture.pdf) devices under Kubernetes.

### DLB2 driver configuration for PFs
The DLB device plugin requires a Linux Kernel DLB driver to be installed and enabled to operate. Get [DLB software release](https://www.intel.com/content/www/us/en/download/686372/intel-dynamic-load-balancer.html), build and load the dlb2 driver module following the instruction of 'DLB_Driver_User_Guide.pdf' in the directory 'dlb/docs'.

After successfully loading the module, available dlb device nodes are visible in devfs.
```bash
$ ls -1 /dev/dlb*
/dev/dlb0  /dev/dlb1  /dev/dlb2 ...
```

### VF configuration using a DPDK tool (but with dlb2 driver)
If you configure SR-IOV/VF (virtual functions), continue the following configurations. This instruction uses DPDK tool to check eventdev devices, unbind a VF device, and bind dlb2 driver to a VF device.

Patch dpdk sources to work with DLB:
```bash
$ wget -q https://fast.dpdk.org/rel/dpdk-22.11.2.tar.xz -O- | tar -Jx
$ wget -q https://downloadmirror.intel.com/791459/dlb_linux_src_release8.5.2.txz -O- | tar -Jx
$ cd ./dpdk-*/ && patch -p1 < ../dlb/dpdk/dpdk_dlb_*_diff.patch
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
dlb dpdk-21.11
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
dlb dpdk-21.11
$ cd ./dpdk-* && meson setup --prefix $(pwd)/installdir builddir && ninja -C builddir install
```

- Run eventdev test
```bash
sudo ./builddir/app/dpdk-test-eventdev --no-huge --vdev='dlb2_event,dev_id=1' -- --test=order_queue --nb_flows 64 --nb_pkts 512 --plcores 1 --wlcores 2-7
# For running test for /dev/dlbN, replace 1 with N.
```

## Installation

The following sections detail how to obtain, build, deploy and test the DLB device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-dlb-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/dlb_plugin?ref=<RELEASE_VERSION>'
daemonset.apps/intel-dlb-plugin created
```

Where `<RELEASE_VERSION>` needs to be substituted with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

Nothing else is needed. See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the plugin.

### Verify Plugin Registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{range $k,$v:=.status.allocatable}}{{"  "}}{{$k}}{{": "}}{{$v}}{{"\n"}}{{end}}{{end}}' | grep '^\([^ ]\)\|\(  dlb\)'
master
  dlb.intel.com/pf: 7
  dlb.intel.com/vf: 4
```

## Testing and Demos

We can test the plugin is working by deploying the provided example test images (dlb-libdlb-demo and dlb-dpdk-demo).

1. Build a Docker image and create a pod running unit tests off the local Docker image:

    ```bash
    $ make dlb-libdlb-demo
    ...
    Successfully tagged intel/dlb-libdlb-demo:devel

    $ kubectl apply -f ${INTEL_DEVICE_PLUGINS_SRC}/demo/dlb-libdlb-demo-pod.yaml
    pod/dlb-libdlb-demo-pod created
    ```

    ```bash
    $ make dlb-dpdk-demo
    ...
    Successfully tagged intel/dlb-dpdk-demo:devel

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
