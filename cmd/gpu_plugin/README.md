# Intel GPU device plugin for Kubernetes

# Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Getting the source code](#getting-the-source-code)
    * [Verify node kubelet config](#verify-node-kubelet-config)
    * [Deploying as a DaemonSet](#deploying-as-a-daemonset)
        * [Build the plugin image](#build-the-plugin-image)
        * [Deploy plugin DaemonSet](#deploy-plugin-daemonset)
    * [Deploy by hand](#deploy-by-hand)
        * [Build the plugin](#build-the-plugin)
        * [Run the plugin as administrator](#run-the-plugin-as-administrator)
    * [Verify plugin registration](#verify-plugin-registration)
    * [Testing the plugin](#testing-the-plugin)

# Introduction

The GPU device plugin for Kubernetes supports Intel
[GVT-d](https://github.com/intel/gvt-linux/wiki/GVTd_Setup_Guide) device passthrough
and acceleration, supporting GPUs of the following hardware families:

- Integrated GPUs within Intel Core processors
- Integrated GPUs within Intel Xeon processors
- Intel Visual Compute Accelerator (Intel VCA)

The GPU plugin facilitates offloading the processing of computation intensive workloads to GPU hardware.
There are two primary use cases:

- hardware vendor-independent acceleration using the [Intel Media SDK](https://github.com/Intel-Media-SDK/MediaSDK)
- OpenCL code tuned for high end Intel devices.

For example, the Intel Media SDK can offload video transcoding operations, and the OpenCL libraries can provide computation acceleration for Intel GPUs

For information on Intel GVT-g virtual GPU device passthrough (as opposed to full device passthrough), see
[this site](https://github.com/intel/gvt-linux/wiki/GVTg_Setup_Guide).

# Installation

The following sections detail how to obtain, build, deploy and test the GPU device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

## Getting the source code

> **Note:** It is presumed you have a valid and configured [golang](https://golang.org/) environment
> that meets the minimum required version.

```bash
$ go get -d -u github.com/intel/intel-device-plugins-for-kubernetes
```

## Verify node kubelet config

Every node that will be running the gpu plugin must have the
[kubelet device-plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
configured. For each node, check that the kubelet device plugin socket exists:

```bash
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

## Deploying as a DaemonSet

To deploy the gpu plugin as a daemonset, you first need to build a container image for the
plugin and ensure that is visible to your nodes.

### Build the plugin image

The following will use `docker` to build a local container image called
`intel/intel-gpu-plugin` with the tag `devel`.

The image build tool can be changed from the default `docker` by setting the `BUILDER` argument
to the [`Makefile`](Makefile).

```bash
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make intel-gpu-plugin
...
Successfully tagged intel/intel-gpu-plugin:devel
```

### Deploy plugin DaemonSet

You can then use the [example DaemonSet YAML](../../deployments/gpu_plugin/base/intel-gpu-plugin.yaml)
file provided to deploy the plugin. The default kustomization that deploys the YAML as is:

```bash
$ kubectl apply -k deployments/gpu_plugin
daemonset.apps/intel-gpu-plugin created
```

Alternatively, if your cluster runs
[Node Feature Discovery](https://github.com/kubernetes-sigs/node-feature-discovery),
you can deploy the device plugin only on nodes with Intel GPU.
The [nfd_labeled_nodes](../../deployments/gpu_plugin/overlays/nfd_labeled_nodes/)
kustomization adds the nodeSelector to the DaemonSet:

```bash
$ kubectl apply -k deployments/gpu_plugin/overlays/nfd_labeled_nodes
daemonset.apps/intel-gpu-plugin created
```

> **Note**: It is also possible to run the GPU device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to device plugin socket creation and kubelet registration.
Furthermore, the deployments `securityContext` must be configured with appropriate `runAsUser/runAsGroup`.

## Deploy by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

### Build the plugin

First we build the plugin:

```bash
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make gpu_plugin
```

### Run the plugin as administrator

Now we can run the plugin directly on the node:

```bash
$ sudo $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/gpu_plugin
device-plugin start server at: /var/lib/kubelet/device-plugins/gpu.intel.com-i915.sock
device-plugin registered
```

## Verify plugin registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o=jsonpath="{range .items[*]}{.metadata.name}{'\n'}{' i915: '}{.status.allocatable.gpu\.intel\.com/i915}{'\n'}"
master
 i915: 1
```

## Testing the plugin

We can test the plugin is working by deploying the provided example OpenCL image with FFT offload enabled.

1. Build a Docker image with an example program offloading FFT computations to GPU:

    ```bash
    $ cd demo
    $ ./build-image.sh ubuntu-demo-opencl
    ...
    Successfully tagged ubuntu-demo-opencl:devel
    ```

1. Create a job running unit tests off the local Docker image:

    ```bash
    $ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
    $ kubectl apply -f demo/intelgpu-job.yaml
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
