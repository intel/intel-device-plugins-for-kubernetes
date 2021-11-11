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


## Installation

The following sections detail how to obtain, build, deploy and test the DLB device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Deploy with pre-built container image
*This part will be updated later if necessary, since the links below do not contain the images at the point when README is written.

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
