# Intel GPU device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Modes and Configuration Options](#modes-and-configuration-options)
* [Installation](#installation)
    * [Prerequisites](#prerequisites)
        * [Drivers for discrete GPUs](#drivers-for-discrete-gpus)
            * [Kernel driver](#kernel-driver)
            * [User-space drivers](#user-space-drivers)
        * [Drivers for older (integrated) GPUs](#drivers-for-older-integrated-gpus)
    * [Pre-built Images](#pre-built-images)
         * [Install to all nodes](#install-to-all-nodes)
         * [Install to nodes with Intel GPUs with NFD](#install-to-nodes-with-intel-gpus-with-nfd)
         * [Install to nodes with Intel GPUs with Fractional resources](#install-to-nodes-with-intel-gpus-with-fractional-resources)
              * [Fractional resources details](#fractional-resources-details)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Testing and Demos](#testing-and-demos)
* [Issues with media workloads on multi-GPU setups](#issues-with-media-workloads-on-multi-gpu-setups)
    * [Workaround for QSV and VA-API](#workaround-for-qsv-and-va-api)


## Introduction

Intel GPU plugin facilitates Kubernetes workload offloading by providing access to
discrete (including Intel® Data Center GPU Flex Series) and integrated Intel GPU devices
supported by the host kernel.

Use cases include, but are not limited to:
- Media transcode
- Media analytics
- Cloud gaming
- High performance computing
- AI training and inference

For example containers with Intel media driver (and components using that), can offload
video transcoding operations, and containers with the Intel OpenCL / oneAPI Level Zero
backend libraries can offload compute operations to GPU.

## Modes and Configuration Options

| Flag | Argument | Default | Meaning |
|:---- |:-------- |:------- |:------- |
| -enable-monitoring | - | disabled | Enable 'i915_monitoring' resource that provides access to all Intel GPU devices on the node |
| -resource-manager | - | disabled | Enable fractional resource management, [see also dependencies](#fractional-resources) |
| -shared-dev-num | int | 1 | Number of containers that can share the same GPU device |
| -allocation-policy | string | none | 3 possible values: balanced, packed, none. It is meaningful when shared-dev-num > 1, balanced mode is suitable for workload balance among GPU devices, packed mode is suitable for making full use of each GPU device, none mode is the default. Allocation policy does not have effect when resource manager is enabled. |

The plugin also accepts a number of other arguments (common to all plugins) related to logging.
Please use the -h option to see the complete list of logging related options.

## Installation

The following sections detail how to obtain, build, deploy and test the GPU device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Prerequisites

Access to a GPU device requires firmware, kernel and user-space
drivers supporting it.  Firmware and kernel driver need to be on the
host, user-space drivers in the GPU workload containers.

Intel GPU devices supported by the current kernel can be listed with:
```
$ grep i915 /sys/class/drm/card?/device/uevent
/sys/class/drm/card0/device/uevent:DRIVER=i915
/sys/class/drm/card1/device/uevent:DRIVER=i915
```

#### Drivers for discrete GPUs

##### Kernel driver

For now, kernel needs to be built from sources. `i915` GPU driver DKMS package
sources for a subset of older kernels in enterprise / LTS distributions, are in
[intel-gpu-i915-backports](https://github.com/intel-gpu/intel-gpu-i915-backports).

With upstream 6.x kernels, discrete GPU support needs to be enabled using
kernel `i915.force_probe=<PCI_ID>` command line option until relevant kernel
driver features have been completed also in upstream:
https://www.kernel.org/doc/html/latest/gpu/rfc/index.html

PCI IDs for the Intel GPUs on given host can be listed with:
```
$ lspci | grep -e VGA -e Display | grep Intel
88:00.0 Display controller: Intel Corporation Device 56c1 (rev 05)
8d:00.0 Display controller: Intel Corporation Device 56c1 (rev 05)
```

(`lspci` lists GPUs with display support as "VGA compatible controller",
and server GPUs without display support, as "Display controller".)

Mesa "Iris" 3D driver header provides a mapping between GPU PCI IDs and their Intel brand names:
https://gitlab.freedesktop.org/mesa/mesa/-/blob/main/include/pci_ids/iris_pci_ids.h

If your kernel build does not find the correct firmware version for
a given GPU from the host (see `dmesg | grep i915` output), latest
firmware versions are available in upstream:
https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/tree/i915

##### User-space drivers

Until new enough user-space drivers (supporting also discrete GPUs)
are available directly from distribution package repositories, they
can be installed to containers from Intel package repositories. See:
https://dgpu-docs.intel.com/installation-guides/index.html

Example container is listed in [Testing and demos](#testing-and-demos).

Validation status against *upstream* kernel is listed in the user-space drivers release notes:
* Media driver: https://github.com/intel/media-driver/releases
* Compute driver: https://github.com/intel/compute-runtime/releases

#### Drivers for older (integrated) GPUs

For the older (integrated) GPUs, new enough firmware and kernel driver
are typically included already with the host OS, and new enough
user-space drivers (for the GPU containers) are in the host OS
repositories.

### Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-gpu-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

Note: Replace `<RELEASE_VERSION>` with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the plugin.

#### Install to all nodes

Simplest option to enable use of Intel GPUs in Kubernetes Pods.

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin?ref=<RELEASE_VERSION>
```

#### Install to nodes with Intel GPUs with NFD

Deploying GPU plugin to only nodes that have Intel GPU attached. [Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery) is required to detect the presence of Intel GPUs.

```bash
# Start NFD - if your cluster doesn't have NFD installed yet
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd?ref=<RELEASE_VERSION>

# Create NodeFeatureRules for detecting GPUs on nodes
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>

# Create GPU plugin daemonset
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin/overlays/nfd_labeled_nodes?ref=<RELEASE_VERSION>
```

#### Install to nodes with Intel GPUs with Fractional resources

With the experimental fractional resource feature you can use additional kubernetes extended
resources, such as GPU memory, which can then be consumed by deployments. PODs will then only
deploy to nodes where there are sufficient amounts of the extended resources for the containers.

(For this to work properly, all GPUs in a given node should provide equal amount of resources
i.e. heteregenous GPU nodes are not supported.)

Enabling the fractional resource feature isn't quite as simple as just enabling the related
command line flag. The DaemonSet needs additional RBAC-permissions
and access to the kubelet podresources gRPC service, plus there are other dependencies to
take care of, which are explained below. For the RBAC-permissions, gRPC service access and
the flag enabling, it is recommended to use kustomization by running:

```bash
# Start NFD with GPU related configuration changes
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/gpu?ref=<RELEASE_VERSION>

# Create NodeFeatureRules for detecting GPUs on nodes
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>

# Create GPU plugin daemonset
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin/overlays/fractional_resources?ref=<RELEASE_VERSION>
```

##### Fractional resources details

Usage of these fractional GPU resources requires that the cluster has node
extended resources with the name prefix `gpu.intel.com/`. Those can be created with NFD
by running the [hook](/cmd/gpu_nfdhook/) installed by the plugin initcontainer. When fractional resources are
enabled, the plugin lets a [scheduler extender](https://github.com/intel/platform-aware-scheduling/tree/master/gpu-aware-scheduling)
do card selection decisions based on resource availability and the amount of extended
resources requested in the [pod spec](https://github.com/intel/platform-aware-scheduling/blob/master/gpu-aware-scheduling/docs/usage.md#pods).

The scheduler extender then needs to annotate the pod objects with unique
increasing numeric timestamps in the annotation `gas-ts` and container card selections in
`gas-container-cards` annotation. The latter has container separator '`|`' and card separator
'`,`'. Example for a pod with two containers and both containers getting two cards:
`gas-container-cards:card0,card1|card2,card3`. Enabling the fractional-resource support
in the plugin without running such an annotation adding scheduler extender in the cluster
will only slow down GPU-deployments, so do not enable this feature unnecessarily.

In multi-tile systems, containers can request individual tiles to improve GPU resource usage.
Tiles targeted for containers are specified to pod via `gas-container-tiles` annotation where the the annotation
value describes a set of card and tile combinations. For example in a two container pod, the annotation
could be `gas-container-tiles:card0:gt0+gt1|card1:gt1,card2:gt0`. Similarly to `gas-container-cards`, the container
details are split via `|`. In the example above, the first container gets tiles 0 and 1 from card 0,
and the second container gets tile 1 from card 1 and tile 0 from card 2.

> **Note**: It is also possible to run the GPU device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to device plugin socket creation and kubelet registration.
Furthermore, the deployments `securityContext` must be configured with appropriate `runAsUser/runAsGroup`.

### Verify Plugin Registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o=jsonpath="{range .items[*]}{.metadata.name}{'\n'}{' i915: '}{.status.allocatable.gpu\.intel\.com/i915}{'\n'}"
master
 i915: 1
```

## Testing and Demos

The GPU plugin functionality can be verified by deploying an [OpenCL image](../../demo/intel-opencl-icd/) which runs `clinfo` outputting the GPU capabilities (detected by driver installed to the image).

1. Make the image available to the cluster:

    Build image:

    ```bash
    $ make intel-opencl-icd
    ```

    Tag and push the `intel-opencl-icd` image to a repository available in the cluster. Then modify the `intelgpu-job.yaml`'s image location accordingly:

    ```bash
    $ docker tag intel/intel-opencl-icd:devel <repository>/intel/intel-opencl-icd:latest
    $ docker push <repository>/intel/intel-opencl-icd:latest
    $ $EDITOR ${INTEL_DEVICE_PLUGINS_SRC}/demo/intelgpu-job.yaml
    ```

    If you are running the demo on a single node cluster, and do not have your own registry, you can add image to node image cache instead. For example, to import docker image to containerd cache:

    ```bash
    $ IMAGE_NAME=opencl-icd.tar
    $ docker save -o $IMAGE_NAME intel/intel-opencl-icd:devel
    $ ctr -n=k8s.io images import $IMAGE_NAME
    $ rm $IMAGE_NAME
    ```

1. Create a job:

    ```bash
    $ kubectl apply -f ${INTEL_DEVICE_PLUGINS_SRC}/demo/intelgpu-job.yaml
    job.batch/intelgpu-demo-job created
    ```

1. Review the job's logs:

    ```bash
    $ kubectl get pods | fgrep intelgpu
    # substitute the 'xxxxx' below for the pod name listed in the above
    $ kubectl logs intelgpu-demo-job-xxxxx
    <log output>
    ```

    If the pod did not successfully launch, possibly because it could not obtain
    the requested GPU resource, it will be stuck in the `Pending` status:

    ```bash
    $ kubectl get pods
    NAME                      READY   STATUS    RESTARTS   AGE
    intelgpu-demo-job-xxxxx   0/1     Pending   0          8s
    ```

    This can be verified by checking the Events of the pod:

    ```bash
    $ kubectl describe pod intelgpu-demo-job-xxxxx
    ...
    Events:
      Type     Reason            Age        From               Message
      ----     ------            ----       ----               -------
      Warning  FailedScheduling  <unknown>  default-scheduler  0/1 nodes are available: 1 Insufficient gpu.intel.com/i915.
    ```


## Issues with media workloads on multi-GPU setups

Unlike with 3D & compute, and OneVPL media API, QSV (MediaSDK) & VA-API
media APIs do not offer device discovery functionality for applications.
There is nothing (e.g. environment variable) with which the default
device could be overridden either.

As result, most (all?) media applications using VA-API or QSV, fail to
locate the correct GPU device file unless it is the first ("renderD128")
one, or device file name is explictly specified with an application option.

Kubernetes device plugins expose only requested number of device
files, and their naming matches host device file names (for several
reasons unrelated to media).  Therefore, on multi-GPU hosts, the only
GPU device file mapped to the media container can be some other one
than "renderD128", and media applications using VA-API or QSV need to
be explicitly told which one to use.

These options differ from application to application.  Relevant FFmpeg
options are documented here:
* VA-API: https://trac.ffmpeg.org/wiki/Hardware/VAAPI
* QSV: https://github.com/Intel-Media-SDK/MediaSDK/wiki/FFmpeg-QSV-Multi-GPU-Selection-on-Linux


### Workaround for QSV and VA-API

[Render device](render-device.sh) shell script locates and outputs the
correct device file name.  It can be added to the container and used
to give device file name for the application.

Use it either from another script invoking the application, or
directly from the Pod YAML command line.  In latter case, it can be
used either to add the device file name to the end of given command
line, like this:

```bash
command: ["render-device.sh", "vainfo", "--display", "drm", "--device"]

=> /usr/bin/vainfo --display drm --device /dev/dri/renderDXXX
```

Or inline, like this:

```bash
command: ["/bin/sh", "-c",
          "vainfo --device $(render-device.sh 1) --display drm"
         ]
```

If device file name is needed for multiple commands, one can use shell variable:

```bash
command: ["/bin/sh", "-c",
          "dev=$(render-device.sh 1) && vainfo --device $dev && <more commands>"
         ]
```

With argument N, script outputs name of the Nth suitable GPU device
file, which can be used when more than one GPU resource was requested.
