# Intel NPU device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Modes and Configuration Options](#modes-and-configuration-options)
* [UMD, KMD and firmware](#umd-kmd-and-firmware)
* [Pre-built Images](#pre-built-images)
* [Installation](#installation)
    * [Install with NFD](#install-with-nfd)
    * [Install with Operator](#install-with-operator)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Testing and Demos](#testing-and-demos)

## Introduction

Intel NPU plugin facilitates Kubernetes workload offloading by providing access to Intel CPU neural processing units supported by the host kernel.

The following CPU families are currently detected by the plugin:
* Core Ultra Series 1 (Meteor Lake)
* Core Ultra Series 2 (Arrow Lake)
* Core Ultra 200V Series (Lunar Lake)
* Core Ultra Series 3 (Panther Lake)

Intel NPU plugin registers a resource to the Kubernetes cluster:
| Resource | Description |
|:---- |:-------- |
| npu.intel.com/accel | NPU |

## Modes and Configuration Options

| Flag | Argument | Default | Meaning |
|:---- |:-------- |:------- |:------- |
| -shared-dev-num | int | 1 | Number of containers that can share the same NPU device |

The plugin also accepts a number of other arguments (common to all plugins) related to logging.
Please use the -h option to see the complete list of logging related options.

## UMD, KMD, and Firmware

To run workloads on the NPU device, three components are required:

- **UMD (User Mode Driver):** Must be included in the container image. Download it from the [Intel NPU driver](https://github.com/intel/linux-npu-driver/) project.
- **KMD (Kernel Mode Driver):** Provided by recent Linux distributions (e.g., Ubuntu 24.04) as part of the operating system.
- **Firmware:** Also included in modern Linux distributions, or available from [linux-firmware](https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/tree/intel/vpu) and [intel-npu-driver](https://github.com/intel/linux-npu-driver/tree/main/firmware/bin).

For a detailed overview, see the Intel NPU driver [documentation](https://github.com/intel/linux-npu-driver/blob/main/docs/overview.md).

An example [demo workload](#testing-and-demos) is provided in this repository.

For reference:
- The NPU KMD source is in the [Linux kernel](https://github.com/torvalds/linux/tree/master/drivers/accel/ivpu).
- Firmware sources are in [linux-firmware](https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/tree/intel/vpu) and [intel-npu-driver](https://github.com/intel/linux-npu-driver/tree/main/firmware/bin).

## Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-npu-plugin)
are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository.

See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the plugin.

## Installation

There are multiple ways to install Intel NPU plugin to a cluster. The most common methods are described below.

> **Note**: Replace `<RELEASE_VERSION>` with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

> **Note**: Add ```--dry-run=client -o yaml``` to the ```kubectl``` commands below to visualize the YAML content being applied.

### Install with NFD

Deploy NPU plugin with the help of NFD ([Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery)). It detects the presence of Intel NPUs and labels them accordingly. NPU plugin's node selector is used to deploy plugin to nodes which have such a NPU label.

```bash
# Start NFD - if your cluster doesn't have NFD installed yet
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd?ref=<RELEASE_VERSION>'

# Create NodeFeatureRules for detecting NPUs on nodes
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/nfd/overlays/node-feature-rules?ref=<RELEASE_VERSION>'

# Create NPU plugin daemonset
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/npu_plugin/overlays/nfd_labeled_nodes?ref=<RELEASE_VERSION>'
```

### Install with Operator

NPU plugin can be installed with the Intel Device Plugin Operator. It allows configuring NPU plugin parameters without kustomizing the deployment files. The general installation is described in the [install documentation](../operator/README.md#installation).

### Verify Plugin Registration

You can verify that the plugin has been installed on the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o=jsonpath="{range .items[*]}{.metadata.name}{'\n'}{' accel: '}{.status.allocatable.npu\.intel\.com/accel}{'\n'}"
master
 accel: 1
```

## Testing and Demos

The NPU plugin functionality can be verified by deploying a [npu-plugin-demo](../../demo/intel-npu-demo/) image which runs tests with the Intel NPU.

1. Make the image available to the cluster:

    Build image:

    ```bash
    $ make intel-npu-demo
    ```

    Tag and push the `intel-npu-demo` image to a repository available in the cluster. Then modify the [intel-npu-workload.yaml's](../../demo/intel-npu-workload.yaml) image location accordingly:

    ```bash
    $ docker tag intel/intel-npu-demo:devel <repository>/intel/intel-npu-demo:latest
    $ docker push <repository>/intel/intel-npu-demo:latest
    $ $EDITOR ${INTEL_DEVICE_PLUGINS_SRC}/demo/intel-npu-workload.yaml
    ```

    If you are running the demo on a single node cluster, and do not have your own registry, you can add image to node image cache instead. For example, to import docker image to containerd cache:

    ```bash
    $ docker save intel/intel-npu-demo:devel | ctr -n k8s.io images import -
    ```
    Running `ctr` may require the use of `sudo`.

1. Create a job:

    ```bash
    $ kubectl apply -f ${INTEL_DEVICE_PLUGINS_SRC}/demo/intel-npu-workload.yaml
    job.batch/npu-workload created
    ```

1. Review the job's logs:

    ```bash
    $ kubectl get pods | fgrep npu-workload
    # substitute the 'xxxxx' below for the pod name listed above
    $ kubectl logs npu-workload-xxxxx
    <log output>
    ```
