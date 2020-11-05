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

## Introduction

The DSA device plugin for Kubernetes supports acceleration using the Intel Data Streaming accelerator(DSA).

The DSA plugin discovers DSA work queues and presents them as a node resources.

## Installation

The following sections detail how to obtain, build, deploy and test the DSA device plugin.

Examples are provided showing how to deploy the plugin either using a DaemonSet or by hand on a per-node basis.

### Deploy with pre-built container image

[Pre-built images](https://hub.docker.com/r/intel/intel-dsa-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest master branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/dsa_plugin?ref=<REF>
daemonset.apps/intel-dsa-plugin created
```

Where `<REF>` needs to be substituted with the desired git ref, e.g. `master`.

Nothing else is needed. But if you want to deploy a customized version of the plugin read further.

### Getting the source code

```bash
$ export INTEL_DEVICE_PLUGINS_SRC=/path/to/intel-device-plugins-for-kubernetes
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes ${INTEL_DEVICE_PLUGINS_SRC}
```

### Verify node kubelet config

Every node that will be running the dsa plugin must have the
[kubelet device-plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
configured. For each node, check that the kubelet device plugin socket exists:

```bash
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
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
$ kubectl get nodes -o=jsonpath="{range .items[*]}{.metadata.name}{'\n'}{' i915: '}{.status.allocatable.dsa\.intel\.com/*}{'\n'}"
master
  dsa.intel.com/wq-user-dedicated:  1
  dsa.intel.com/wq-user-shared:     1
```
