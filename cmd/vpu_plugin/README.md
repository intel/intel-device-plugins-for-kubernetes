# Intel VPU device plugin for Kubernetes

Table of Contents

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
        * [Build a Docker image with an classification example](#build-a-docker-image-with-an-classification-example)
        * [Create a job running unit tests off the local Docker image](#create-a-job-running-unit-tests-off-the-local-docker-image)
        * [Review the job logs](#review-the-job-logs)

## Introduction

The VPU device plugin supports below cards:

[Intel VCAC-A](https://www.intel.com/content/dam/www/public/us/en/documents/datasheets/media-analytics-vcac-a-accelerator-card-by-celestica-datasheet.pdf).
This card has:
- 1 Intel Core i3-7100U processor
- 12 MyriadX VPUs
- 8GB DDR4 memory
- PCIe interface to Xeon E3/E5 server

[Intel Mustang V100](https://software.intel.com/en-us/articles/introducing-the-iei-tank-aiot-developer-kit-and-mustang-v100-mx8-pcie-accelerator-card).
This card has:
- 8 MyriadX VPUs
- PCIe interface to 6th+ Generation Core PC or Xeon E3/E5 server

> **Note:** This device plugin need HDDL daemon service to be running either natively or from a container.
>      To get VCAC-A or Mustang card running hddl, please refer to:
> https://github.com/OpenVisualCloud/Dockerfiles/blob/master/VCAC-A/script/setup_hddl.sh

## Installation

The following sections detail how to obtain, build, deploy and test the VPU device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Getting the source code

> **Note:** It is presumed you have a valid and configured [golang](https://golang.org/) environment
> that meets the minimum required version.

```bash
$ mkdir -p $(go env GOPATH)/src/github.com/intel
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
```

### Deploying as a DaemonSet

To deploy the vpu plugin as a daemonset, you first need to build a container image for the
plugin and ensure that is visible to your nodes.

#### Build the plugin image

The following will use `docker` to build a local container image called
`intel/intel-vpu-plugin` with the tag `devel`.

The image build tool can be changed from the default `docker` by setting the `BUILDER` argument
to the [`Makefile`](/Makefile).

```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make intel-vpu-plugin
...
Successfully tagged intel/intel-vpu-plugin:devel
```

#### Deploy plugin DaemonSet

You can then use the [example DaemonSet YAML](/deployments/vpu_plugin/base/intel-vpu-plugin.yaml)
file provided to deploy the plugin. The default kustomization that deploys the YAML as is:

```bash
$ kubectl apply -k deployments/vpu_plugin
daemonset.apps/intel-vpu-plugin created
```

> **Note**: It is also possible to run the VPU device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to device plugin socket creation and kubelet registration.
Furthermore, the deployments `securityContext` must be configured with appropriate `runAsUser/runAsGroup`.

### Deploy by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

#### Build the plugin

First we build the plugin:

> **Note:** this vpu plugin has dependency of libusb-1.0-0-dev, you need install it before building vpu plugin

```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make vpu_plugin
```

#### Run the plugin as administrator

Now we can run the plugin directly on the node:

```bash
$ sudo $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/vpu_plugin/vpu_plugin
VPU device plugin started
```

### Verify plugin registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o=jsonpath="{range .items[*]}{.metadata.name}{'\n'}{' hddl: '}{.status.allocatable.vpu\.intel\.com/hddl}{'\n'}"
vcaanode00
 hddl: 12
```

### Testing the plugin

We can test the plugin is working by deploying the provided example OpenVINO image with HDDL plugin enabled.

#### Build a Docker image with an classification example

```bash
$ cd demo
$ ./build-image.sh ubuntu-demo-openvino
...
Successfully tagged ubuntu-demo-openvino:devel
```

#### Create a job running unit tests off the local Docker image

```bash
$ cd $(go env GOPATH)/src/github.com/intel/intel-device-plugins-for-kubernetes
$ kubectl apply -f demo/intelvpu-job.yaml
job.batch/intelvpu-demo-job created
```

#### Review the job logs

```bash
$ kubectl get pods | fgrep intelvpu
# substitute the 'xxxxx' below for the pod name listed in the above
$ kubectl logs intelvpu-demo-job-xxxxx
+ export HDDL_INSTALL_DIR=/root/hddl
+ HDDL_INSTALL_DIR=/root/hddl
+ export LD_LIBRARY_PATH=/root/inference_engine_samples_build/intel64/Release/lib/
+ LD_LIBRARY_PATH=/root/inference_engine_samples_build/intel64/Release/lib/
+ /root/inference_engine_samples_build/intel64/Release/classification_sample_async -m /root/openvino_models/ir/FP16/classification/squeezenet/1.1/caffe/squeezenet1.1.xml -i /root/car.png -d HDDL
[ INFO ] InferenceEngine:
    API version ............ 2.0
    Build .................. custom_releases/2019/R2_f5827d4773ebbe727c9acac5f007f7d94dd4be4e
    Description ....... API
[ INFO ] Parsing input parameters
[ INFO ] Parsing input parameters
[ INFO ] Files were added: 1
[ INFO ]     /root/car.png
[ INFO ] Creating Inference Engine
    HDDL
    HDDLPlugin version ......... 2.0
    Build ........... 27579

[ INFO ] Loading network files
[ INFO ] Preparing input blobs
[ WARNING ] Image is resized from (787, 259) to (227, 227)
[ INFO ] Batch size is 1
[ INFO ] Loading model to the device
[07:49:01.0427][6]I[ServiceStarter.cpp:40] Info: Waiting for HDDL Service getting ready ...
[07:49:01.0428][6]I[ServiceStarter.cpp:45] Info: Found HDDL Service is running.
[HDDLPlugin] [07:49:01.0429][6]I[HddlClient.cpp:256] Hddl api version: 2.2
[HDDLPlugin] [07:49:01.0429][6]I[HddlClient.cpp:259] Info: Create Dispatcher2.
[HDDLPlugin] [07:49:01.0432][10]I[Dispatcher2.cpp:148] Info: SenderRoutine starts.
[HDDLPlugin] [07:49:01.0432][6]I[HddlClient.cpp:270] Info: RegisterClient HDDLPlugin.
[HDDLPlugin] [07:49:01.0435][6]I[HddlClient.cpp:275] Client Id: 3
[ INFO ] Create infer request
[HDDLPlugin] [07:49:01.7235][6]I[HddlBlob.cpp:166] Info: HddlBlob initialize ion ...
[HDDLPlugin] [07:49:01.7237][6]I[HddlBlob.cpp:176] Info: HddlBlob initialize ion successfully.
[ INFO ] Start inference (10 asynchronous executions)
[ INFO ] Completed 1 async request execution
[ INFO ] Completed 2 async request execution
[ INFO ] Completed 3 async request execution
[ INFO ] Completed 4 async request execution
[ INFO ] Completed 5 async request execution
[ INFO ] Completed 6 async request execution
[ INFO ] Completed 7 async request execution
[ INFO ] Completed 8 async request execution
[ INFO ] Completed 9 async request execution
[ INFO ] Completed 10 async request execution
[ INFO ] Processing output blobs

Top 10 results:

Image /root/car.png

classid probability label
------- ----------- -----
817     0.8295898   sports car, sport car
511     0.0961304   convertible
479     0.0439453   car wheel
751     0.0101318   racer, race car, racing car
436     0.0074234   beach wagon, station wagon, wagon, estate car, beach waggon, station waggon, waggon
656     0.0042267   minivan
586     0.0029869   half track
717     0.0018148   pickup, pickup truck
864     0.0013924   tow truck, tow car, wrecker
581     0.0006595   grille, radiator grille

[HDDLPlugin] [07:49:01.9231][11]I[Dispatcher2.cpp:212] Info: Listen Thread wake up and to exit.
[HDDLPlugin] [07:49:01.9232][6]I[Dispatcher2.cpp:81] Info: Client dispatcher exit.
[HDDLPlugin] [07:49:01.9235][6]I[HddlClient.cpp:203] Info: Hddl client unregistered.
[ INFO ] Execution successful

[ INFO ] This sample is an API example, for any performance measurements please use the dedicated benchmark_app tool
```

If the pod did not successfully launch, possibly because it could not obtain the vpu HDDL
resource, it will be stuck in the `Pending` status:

```bash
$ kubectl get pods
NAME                      READY   STATUS    RESTARTS   AGE
intelvpu-demo-job-xxxxx   0/1     Pending   0          8s
```

This can be verified by checking the Events of the pod:

```bash
$ kubectl describe pod intelvpu-demo-job-xxxxx
...
Events:
Type     Reason            Age        From               Message
----     ------            ----       ----               -------
Warning  FailedScheduling  <unknown>  default-scheduler  0/1 nodes are available: 1 Insufficient vpu.intel.com/hddl.
```
