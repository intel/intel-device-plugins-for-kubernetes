# Intel GPU device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
    * [Configuration options](#configuration-options)
* [Installation](#installation)
    * [Deploy with pre-built container image](#deploy-with-pre-built-container-image)
    * [Getting the source code](#getting-the-source-code)
    * [Deploying as a DaemonSet](#deploying-as-a-daemonset)
        * [Build the plugin image](#build-the-plugin-image)
        * [Deploy plugin DaemonSet](#deploy-plugin-daemonset)
        * [Fractional resources](#fractional-resources)
    * [Deploy by hand](#deploy-by-hand)
        * [Build the plugin](#build-the-plugin)
        * [Run the plugin as administrator](#run-the-plugin-as-administrator)
    * [Verify plugin registration](#verify-plugin-registration)
    * [Testing the plugin](#testing-the-plugin)

## Introduction

Intel GPU plugin facilitates Kubernetes workload offloading by providing access to
Intel discrete (Xe) and integrated GPU HW device files.

Use cases include, but are not limited to:
- Media transcode 
- Media analytics
- Cloud gaming
- High performance computing
- AI training and inference

For example containers with Intel media driver (and components using that), can offload
video transcoding operations, and containers with the Intel OpenCL / oneAPI Level Zero
backend libraries can offload compute operations to GPU.

### Configuration options

| Flag | Argument | Default | Meaning |
|:---- |:-------- |:------- |:------- |
| -enable-monitoring | - | disabled | Enable 'i915_monitoring' resource that provides access to all Intel GPU devices on the node |
| -resource-manager | - | disabled | Enable fractional resource management, [see also dependencies](#fractional-resources) |
| -shared-dev-num | int | 1 | Number of containers that can share the same GPU device |

The plugin also accepts a number of other arguments (common to all plugins) related to logging.
Please use the -h option to see the complete list of logging related options.

## Installation

The following sections detail how to obtain, build, deploy and test the GPU device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Deploy with pre-built container image

[Pre-built images](https://hub.docker.com/r/intel/intel-gpu-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin?ref=<RELEASE_VERSION>
daemonset.apps/intel-gpu-plugin created
```

Where `<RELEASE_VERSION>` needs to be substituted with the desired release version, e.g. `v0.18.0`.

Alternatively, if your cluster runs
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery),
you can deploy the device plugin only on nodes with Intel GPU.
The [nfd_labeled_nodes](../../deployments/gpu_plugin/overlays/nfd_labeled_nodes/)
kustomization adds the nodeSelector to the DaemonSet:

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/gpu_plugin/overlays/nfd_labeled_nodes?ref=<RELEASE_VERSION>
daemonset.apps/intel-gpu-plugin created
```

Nothing else is needed. But if you want to deploy a customized version of the plugin read further.

### Getting the source code

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Deploying as a DaemonSet

To deploy the gpu plugin as a daemonset, you first need to build a container image for the
plugin and ensure that is visible to your nodes.

#### Build the plugin image

The following will use `docker` to build a local container image called
`intel/intel-gpu-plugin` with the tag `devel`.

The image build tool can be changed from the default `docker` by setting the `BUILDER` argument
to the [`Makefile`](Makefile).

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make intel-gpu-plugin
...
Successfully tagged intel/intel-gpu-plugin:devel
```

#### Deploy plugin DaemonSet

You can then use the [example DaemonSet YAML](/deployments/gpu_plugin/base/intel-gpu-plugin.yaml)
file provided to deploy the plugin. The default kustomization that deploys the YAML as is:

```bash
$ kubectl apply -k deployments/gpu_plugin
daemonset.apps/intel-gpu-plugin created
```

Alternatively, if your cluster runs
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery),
you can deploy the device plugin only on nodes with Intel GPU.
The [nfd_labeled_nodes](/deployments/gpu_plugin/overlays/nfd_labeled_nodes/)
kustomization adds the nodeSelector to the DaemonSet:

```bash
$ kubectl apply -k deployments/gpu_plugin/overlays/nfd_labeled_nodes
daemonset.apps/intel-gpu-plugin created
```

#### Fractional resources

With the experimental fractional resource feature you can use additional kubernetes extended
resources, such as GPU memory, which can then be consumed by deployments. PODs will then only
deploy to nodes where there are sufficient amounts of the extended resources for the containers.

Enabling the fractional resource feature isn't quite as simple as just enabling the related
command line flag. The DaemonSet needs additional RBAC-permissions
and access to the kubelet podresources gRPC service, plus there are other dependencies to
take care of, which are explained below. For the RBAC-permissions, gRPC service access and
the flag enabling, it is recommended to use kustomization by running:

```bash
$ kubectl apply -k deployments/gpu_plugin/overlays/fractional_resources
serviceaccount/resource-reader-sa created
clusterrole.rbac.authorization.k8s.io/resource-reader created
clusterrolebinding.rbac.authorization.k8s.io/resource-reader-rb created
daemonset.apps/intel-gpu-plugin created
```

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

> **Note**: It is also possible to run the GPU device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to device plugin socket creation and kubelet registration.
Furthermore, the deployments `securityContext` must be configured with appropriate `runAsUser/runAsGroup`.

### Deploy by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

#### Build the plugin

First we build the plugin:

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make gpu_plugin
```

#### Run the plugin as administrator

Now we can run the plugin directly on the node:

```bash
$ sudo -E ${INTEL_DEVICE_PLUGINS_SRC}/cmd/gpu_plugin/gpu_plugin
device-plugin start server at: /var/lib/kubelet/device-plugins/gpu.intel.com-i915.sock
device-plugin registered
```

### Verify plugin registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o=jsonpath="{range .items[*]}{.metadata.name}{'\n'}{' i915: '}{.status.allocatable.gpu\.intel\.com/i915}{'\n'}"
master
 i915: 1
```

### Testing the plugin

We can test the plugin is working by deploying the provided example OpenCL image with FFT offload enabled.

1. Build a Docker image with an example program offloading FFT computations to GPU:

    ```bash
    $ cd ${INTEL_DEVICE_PLUGINS_SRC}/demo
    $ ./build-image.sh ubuntu-demo-opencl
    ...
    Successfully tagged ubuntu-demo-opencl:devel
    ```

1. Create a job running unit tests off the local Docker image:

    ```bash
    $ kubectl apply -f ${INTEL_DEVICE_PLUGINS_SRC}/demo/intelgpu-job.yaml
    job.batch/intelgpu-demo-job created
    ```

1. Review the job's logs:

    ```bash
    $ kubectl get pods | fgrep intelgpu
    # substitute the 'xxxxx' below for the pod name listed in the above
    $ kubectl logs intelgpu-demo-job-xxxxx
    + WORK_DIR=/root/6-1/fft
    + cd /root/6-1/fft
    + ./fft
    + uprightdiff --format json output.pgm /expected.pgm diff.pgm
    + cat diff.json
    + jq .modifiedArea
    + MODIFIED_AREA=0
    + [ 0 -gt 10 ]
    + echo Success
    Success
    ```

    If the pod did not successfully launch, possibly because it could not obtain the gpu
    resource, it will be stuck in the `Pending` status:

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
