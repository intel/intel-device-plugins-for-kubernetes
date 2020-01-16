# Intel GPU device plugin for Kubernetes

# Table of Contents


* [Introduction](#introduction)
    * [Build and test](#build-and-test)
        * [Getting the source code:](#getting-the-source-code)
        * [Verify kubelet socket exists in /var/lib/kubelet/device-plugins/ directory:](#verify-kubelet-socket-exists-in-varlibkubeletdevice-plugins-directory)
        * [Deploy GPU device plugin as host process for development purposes](#deploy-gpu-device-plugin-as-host-process-for-development-purposes)
            * [Build GPU device plugin:](#build-gpu-device-plugin)
            * [Run GPU device plugin as administrator:](#run-gpu-device-plugin-as-administrator)
        * [Deploy GPU device plugin as a DaemonSet:](#deploy-gpu-device-plugin-as-a-daemonset)
            * [Build plugin image](#build-plugin-image)
            * [Create plugin DaemonSet](#create-plugin-daemonset)
        * [Verify GPU device plugin is registered on master:](#verify-gpu-device-plugin-is-registered-on-master)
        * [Test GPU device plugin:](#test-gpu-device-plugin)

# Introduction

The GPU device plugin for Kubernetes supports Intel
[GVT-d](https://github.com/intel/gvt-linux/wiki/GVTd_Setup_Guide) device passthrough
and acceleration, supporting GPUs of the following hardware families:

- Integrated GPUs within Intel Core processors
- Integrated GPUs within Intel Xeon processors
- Intel Visual Compute Accelerator (Intel VCA)

The GPU plugin offloads the processing of computation intensive workloads to GPU hardware.
There are two primary use cases:

- hardware vendor-independent acceleration using the [Intel Media SDK](https://github.com/Intel-Media-SDK/MediaSDK)
- OpenCL code tuned for high end Intel devices.

For example, the Intel Media SDK can offload video transcoding operations, and the OpenCL™ libraries can provide computation acceleration for Intel GPUs

For information on Intel GVT-g virtual GPU device passthrough, see
[this site](https://github.com/intel/gvt-linux/wiki/GVTg_Setup_Guide).

## Build and test

The following sections detail how to obtain, build, test and deploy the GPU device plugin.

### Getting the source code:

```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Verify kubelet socket exists in /var/lib/kubelet/device-plugins/ directory:
```
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

### Deploy GPU device plugin as host process for development purposes

#### Build GPU device plugin:
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make gpu_plugin
```

#### Run GPU device plugin as administrator:
```
$ sudo $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/gpu_plugin
device-plugin start server at: /var/lib/kubelet/device-plugins/gpu.intel.com-i915.sock
device-plugin registered
```

### Deploy GPU device plugin as a DaemonSet:

#### Build plugin image
```
$ make intel-gpu-plugin
```

#### Create plugin DaemonSet
```
$ kubectl create -f ./deployments/gpu_plugin/gpu_plugin.yaml
daemonset.apps/intel-gpu-plugin created
```

**Note**: It is also possible to run the GPU device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to device plugin socket creation and kubelet registration.
Furthermore, the deployments `securityContext` must be configured with appropriate `runAsUser/runAsGroup`.

### Verify GPU device plugin is registered on master:
```
$ kubectl describe node <node name> | grep gpu.intel.com
 gpu.intel.com/i915:  1
 gpu.intel.com/i915:  1
```

### Test GPU device plugin:

1. Build a Docker image with an example program offloading FFT computations to GPU:
   ```
   $ cd demo
   $ ./build-image.sh ubuntu-demo-opencl
   ```

      This command produces a Docker image named `ubuntu-demo-opencl`.

2. Create a pod running unit tests off the local Docker image:
   ```
   $ kubectl apply -f demo/intelgpu-job.yaml
   ```

3. Review the pod's logs:
   ```
   $ kubectl logs intelgpu-demo-job-xxxx
   ```
