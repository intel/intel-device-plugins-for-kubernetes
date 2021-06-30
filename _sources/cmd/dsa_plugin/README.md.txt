# Intel DSA device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Deploy with pre-built container image](#deploy-with-pre-built-container-image)
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

## Introduction

The DSA device plugin for Kubernetes supports acceleration using the Intel Data Streaming accelerator(DSA).

The DSA plugin discovers DSA work queues and presents them as a node resources.

## Installation

The following sections detail how to obtain, build, deploy and test the DSA device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Deploy with pre-built container image

[Pre-built images](https://hub.docker.com/r/intel/intel-dsa-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/dsa_plugin?ref=<REF>
daemonset.apps/intel-dsa-plugin created
```

Where `<REF>` needs to be substituted with the desired git ref, e.g. `main`.

Nothing else is needed. But if you want to deploy a customized version of the plugin read further.

### Getting the source code

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Deploying as a DaemonSet

To deploy the dsa plugin as a daemonset, you first need to build a container image for the
plugin and ensure that is visible to your nodes.

#### Build the plugin image

The following will use `docker` to build a local container image called
`intel/intel-dsa-plugin` with the tag `devel`.

The image build tool can be changed from the default `docker` by setting the `BUILDER` argument
to the [`Makefile`](Makefile).

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make intel-dsa-plugin
...
Successfully tagged intel/intel-dsa-plugin:devel
```

#### Deploy plugin DaemonSet

You can then use the [example DaemonSet YAML](/deployments/dsa_plugin/base/intel-dsa-plugin.yaml)
file provided to deploy the plugin. The default kustomization that deploys the YAML as is:

```bash
$ kubectl apply -k deployments/dsa_plugin
daemonset.apps/intel-dsa-plugin created
```

### Deploy by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

#### Build the plugin

First we build the plugin:

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make dsa_plugin
```

#### Run the plugin as administrator

Now we can run the plugin directly on the node:

```bash
$ sudo -E ${INTEL_DEVICE_PLUGINS_SRC}/cmd/dsa_plugin/dsa_plugin
device-plugin registered
```

### Verify plugin registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{range $k,$v:=.status.allocatable}}{{"  "}}{{$k}}{{": "}}{{$v}}{{"\n"}}{{end}}{{end}}' | grep '^\([^ ]\)\|\(  dsa\)'
master
  dsa.intel.com/wq-user-dedicated: 2
  dsa.intel.com/wq-user-shared: 8
node1
 dsa.intel.com/wq-user-dedicated: 4
 dsa.intel.com/wq-user-shared: 20
```

### Testing the plugin

We can test the plugin is working by deploying the provided example accel-config test image.

1. Build a Docker image with an accel-config tests:

    ```bash
    $ make dsa-accel-config-demo
    ...
    Successfully tagged dsa-accel-config-demo:devel
    ```

1. Create a pod running unit tests off the local Docker image:

    ```bash
    $ kubectl apply -f ${INTEL_DEVICE_PLUGINS_SRC}/demo/dsa-accel-config-demo-pod.yaml
    pod/dsa-accel-config-demo created
    ```

1. Wait until pod is completed:

    ```bash
    $ kubectl get pods  |grep dsa-accel-config-demo
    dsa-accel-config-demo    0/1     Completed   0          31m

1. Review the job's logs:

    ```bash
    $ kubectl logs dsa-accel-config-demo | tail
    [debug] PF in sub-task[6], consider as passed
    [debug] PF in sub-task[7], consider as passed
    [debug] PF in sub-task[8], consider as passed
    [debug] PF in sub-task[9], consider as passed
    [debug] PF in sub-task[10], consider as passed
    [debug] PF in sub-task[11], consider as passed
    [debug] PF in sub-task[12], consider as passed
    [debug] PF in sub-task[13], consider as passed
    [debug] PF in sub-task[14], consider as passed
    [debug] PF in sub-task[15], consider as passed
    ```

    If the pod did not successfully launch, possibly because it could not obtain the DSA
    resource, it will be stuck in the `Pending` status:

    ```bash
    $ kubectl get pods
    NAME                      READY   STATUS    RESTARTS   AGE
    dsa-accel-config-demo     0/1     Pending   0          7s
    ```

    This can be verified by checking the Events of the pod:

    ```bash

    $ kubectl describe pod dsa-accel-config-demo | grep -A3 Events:
    Events:
      Type     Reason            Age    From               Message
      ----     ------            ----   ----               -------
      Warning  FailedScheduling  2m26s  default-scheduler  0/1 nodes are available: 1 Insufficient dsa.intel.com/wq-user-dedicated, 1 Insufficient dsa.intel.com/wq-user-shared.
    ```
