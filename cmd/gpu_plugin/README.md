# Intel GPU device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Modes and Configuration Options](#modes-and-configuration-options)
* [Operation modes for different workload types](#operation-modes-for-different-workload-types)
* [Installing driver and firmware for Intel GPUs](#installing-driver-and-firmware-for-intel-gpus)
* [Pre-built Images](#pre-built-images)
* [Installation](#installation)
    * [Install with NFD](#install-with-nfd)
    * [Install with Operator](#install-with-operator)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Testing and Demos](#testing-and-demos)
* [Notes](#notes)
  * [Running GPU plugin as non-root](#running-gpu-plugin-as-non-root)
  * [Labels created by GPU plugin](#labels-created-by-gpu-plugin)
  * [SR-IOV use with the plugin](#sr-iov-use-with-the-plugin)
  * [Issues with media workloads on multi-GPU setups](#issues-with-media-workloads-on-multi-gpu-setups)
    * [Workaround for QSV and VA-API](#workaround-for-qsv-and-va-api)

## Introduction

Intel GPU plugin facilitates Kubernetes workload offloading by providing access to
discrete (including Intel® Data Center GPU Flex & Max Series) and integrated Intel GPU devices
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
| -resource-manager | - | disabled | Enable fractional resource management, [see use](./fractional.md) |
| -shared-dev-num | int | 1 | Number of containers that can share the same GPU device |
| -allocation-policy | string | none | 3 possible values: balanced, packed, none. For shared-dev-num > 1: _balanced_ mode spreads workloads among GPU devices, _packed_ mode fills one GPU fully before moving to next, and _none_ selects first available device from kubelet. Default is _none_. Allocation policy does not have an effect when resource manager is enabled. |

The plugin also accepts a number of other arguments (common to all plugins) related to logging.
Please use the -h option to see the complete list of logging related options.

## Operation modes for different workload types

<img src="usage-scenarios.png"/>

Intel GPU-plugin supports a few different operation modes. Depending on the workloads the cluster is running, some modes make more sense than others. Below is a table that explains the differences between the modes and suggests workload types for each mode. Mode selection applies to the whole GPU plugin deployment, so it is a cluster wide decision.

| Mode | Sharing | Intended workloads | Suitable for time critical workloads |
|:---- |:-------- |:------- |:------- |
| shared-dev-num == 1 | No, 1 container per GPU | Workloads using all GPU capacity, e.g. AI training | Yes |
| shared-dev-num > 1 | Yes, >1 containers per GPU | (Batch) workloads using only part of GPU resources, e.g. inference, media transcode/analytics, or CPU bound GPU workloads | No |
| shared-dev-num > 1 && resource-management | Depends on resource requests | Any. For requirements and usage, see [fractional resource management](./fractional.md) | Yes. 1000 millicores = exclusive GPU usage. See note below. |

> **Note**: Exclusive GPU usage with >=1000 millicores requires that also *all other GPU containers* specify (non-zero) millicores resource usage.

## Installing driver and firmware for Intel GPUs

In case your host's operating system lacks support for Intel GPUs, see this page for help: [Drivers for Intel GPUs](./driver-firmware.md)

## Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-gpu-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository.

See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the plugin.

## Installation

There are multiple ways to install Intel GPU plugin to a cluster. The most common methods are described below. For alternative methods, see [advanced install](./advanced-install.md) page.

> **Note**: Replace `<RELEASE_VERSION>` with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

> **Note**: Add ```--dry-run=client -o yaml``` to the ```kubectl``` commands below to visualize the yaml content being applied.

### Install with NFD

Deploy GPU plugin with the help of NFD ([Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)). It detects the presence of Intel GPUs and labels them accordingly. GPU plugin's node selector is used to deploy plugin to nodes which have such a GPU label.

```bash
# Start NFD - if your cluster doesn't have NFD installed yet
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd?ref=<RELEASE_VERSION>'

# Create NodeFeatureRules for detecting GPUs on nodes
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>'

# Create GPU plugin daemonset
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin/overlays/nfd_labeled_nodes?ref=<RELEASE_VERSION>'
```

### Install with Operator

GPU plugin can be installed with the Intel Device Plugin Operator. It allows configuring GPU plugin's parameters without kustomizing the deployment files. The general installation is described in the [install documentation](../operator/README.md#installation). For configuring the GPU Custom Resource (CR), see the [configuration options](#modes-and-configuration-options) and [operation modes](#operation-modes-for-different-workload-types).

### Install alongside with GPU Aware Scheduling

GPU plugin can be installed alongside with GPU Aware Scheduling (GAS). It allows scheduling Pods which e.g. request only partial use of a GPU. The installation is described in [fractional resources](./fractional.md) page.

### Verify Plugin Installation

You can verify that the plugin has been installed on the expected nodes by searching for the relevant
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

## Notes

### Running GPU plugin as non-root

It is possible to run the GPU device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to device plugin socket creation and kubelet registration.
Furthermore, the deployments `securityContext` must be configured with appropriate `runAsUser/runAsGroup`.

More info: https://kubernetes.io/blog/2021/11/09/non-root-containers-and-devices/

### Labels created by GPU plugin

If installed with NFD and started with resource-management, plugin will export a set of labels for the node. For detailed info, see [labeling documentation](./labels.md).

### SR-IOV use with the plugin

GPU plugin does __not__ setup SR-IOV. It has to be configured by the cluster admin.

GPU plugin does however support provisioning Virtual Functions (VFs) to containers for a SR-IOV enabled GPU. When the plugin detects a GPU with SR-IOV VFs configured, it will only provision the VFs and leaves the PF device on the host.

### Issues with media workloads on multi-GPU setups

OneVPL media API, 3D and compute APIs provide device discovery
functionality for applications and work fine in multi-GPU setups.
VA-API and legacy QSV (MediaSDK) media APIs do not, and do not
provide (e.g. environment variable) override for their _default_
device file.

As result, media applications using VA-API or QSV, fail to locate the
correct GPU device file unless it is the first ("renderD128") one, or
device file name is explicitly specified with an application option.

Kubernetes device plugins expose only requested number of device
files, and their naming matches host device file names (for several
reasons unrelated to media).  Therefore, on multi-GPU hosts, the only
GPU device file mapped to the media container can differ from
"renderD128", and media applications using VA-API or QSV need to be
explicitly told which one to use.

These options differ from application to application.  Relevant FFmpeg
options are documented here:
* VA-API: https://trac.ffmpeg.org/wiki/Hardware/VAAPI
* QSV: https://github.com/Intel-Media-SDK/MediaSDK/wiki/FFmpeg-QSV-Multi-GPU-Selection-on-Linux


#### Workaround for QSV and VA-API

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
