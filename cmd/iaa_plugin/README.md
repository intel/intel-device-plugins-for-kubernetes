# Intel IAA device plugin for Kubernetes

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

The IAA device plugin for Kubernetes supports acceleration using the Intel Analytics accelerator(IAA).

The IAA plugin discovers IAA work queues and presents them as a node resources.

The IAA plugin and operator optionally support provisioning of IAA devices and workqueues with the help of [accel-config](https://github.com/intel/idxd-config) utility through initcontainer.

## Installation

The following sections detail how to obtain, build, deploy and test the IAA device plugin.

### Getting the source code

```bash
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes
```

### Deploying as a DaemonSet

To deploy the IAA plugin as a daemonset, you first need to build a container image for the
plugin and ensure that is visible to your nodes.

#### Build the plugin image

The following will use `docker` to build a local container image called
`intel/intel-iaa-plugin` with the tag `devel`.

```bash
$ cd ${INTEL_DEVICE_PLUGINS_SRC}
$ make intel-iaa-plugin
...
Successfully tagged intel/intel-iaa-plugin:devel
```

#### Deploy plugin DaemonSet

You can then use the [example DaemonSet YAML](/deployments/iaa_plugin/base/intel-iaa-plugin.yaml)
file provided to deploy the plugin. The default kustomization that deploys the YAML as is:

```bash
$ kubectl apply -k deployments/iaa_plugin
daemonset.apps/intel-iaa-plugin created
```

### Deploy with initcontainer

There's a sample [idxd initcontainer](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/build/docker/intel-idxd-initcontainer.Dockerfile) included that provisions IAA devices and workqueues (1 engine / 1 group / 1 wq (user/dedicated)), to deploy:

```bash
$ kubectl apply -k deployments/iaa_plugin/overlays/iaa_initcontainer/
```

The provisioning [script](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/idxd-init.sh) and [template](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/iaa.conf) are available for customization.

### Deploy with initcontainer and provisioning config in the ConfigMap

The provisioning config can be optionally stored in the ProvisioningConfig configMap which is then passed to initcontainer through the volume mount.

There's also a possibility for a node specific congfiguration through passing a nodename via NODE_NAME into initcontainer's environment and passing a node specific profile via configMap volume mount.

To create a custom provisioning config:

```bash
$ kubectl create configmap --namespace=inteldeviceplugins-system intel-iaa-config --from-file=demo/iaa.conf --from-file=demo/iaa-node1.conf --dry-run=client -o yaml > iaa-config.yaml
```

### Deploy by hand

For development purposes, it is sometimes convenient to deploy the plugin 'by hand' on a node.
In this case, you do not need to build the complete container image, and can build just the plugin.

#### Build the plugin

First we build the plugin:

```bash
$ make iaa_plugin
```

#### Run the plugin as administrator

Now we can run the plugin directly on the node:

```bash
$ sudo -E ./cmd/iaa_plugin/iaa_plugin
device-plugin registered
```

### Verify plugin registration

You can verify the plugin has been registered with the expected nodes by searching for the relevant
resource allocation status on the nodes:

```bash
$ kubectl get nodes -o go-template='{{range .items}}{{.metadata.name}}{{"\n"}}{{range $k,$v:=.status.allocatable}}{{"  "}}{{$k}}{{": "}}{{$v}}{{"\n"}}{{end}}{{end}}' | grep '^\([^ ]\)\|\(  iaa\)'
master
  iaa.intel.com/wq-user-dedicated: 2
  iaa.intel.com/wq-user-shared: 10
node1
 iaa.intel.com/wq-user-dedicated: 4
 iaa.intel.com/wq-user-shared: 30
```

### Testing the plugin

We can test the plugin is working by deploying the provided example iaa-qpl-demo test image.

1. Build a Docker image with an accel-config tests:

    ```bash
    $ make iaa-qpl-demo
    ...
    Successfully tagged iaa-qpl-demo:devel
    ```

1. Create a pod running unit tests off the local Docker image:

    ```bash
    $ kubectl apply -f ./demo/iaa-qpl-demo-pod.yaml
    pod/iaa-qpl-demo created
    ```

1. Wait until pod is completed:

    ```bash
    $ kubectl get pods  |grep iaa-qpl-demo
    iaa-qpl-demo    0/1     Completed   0          31m

    If the pod did not successfully launch, possibly because it could not obtain the IAA
    resource, it will be stuck in the `Pending` status:

    ```bash
    $ kubectl get pods
    NAME                      READY   STATUS    RESTARTS   AGE
    iaa-qpl-demo              0/1     Pending   0          7s
    ```

    This can be verified by checking the Events of the pod:

    ```bash

    $ kubectl describe pod iaa-qpl-demo | grep -A3 Events:
    Events:
      Type     Reason            Age    From               Message
      ----     ------            ----   ----               -------
      Warning  FailedScheduling  2m26s  default-scheduler  0/1 nodes are available: 1 Insufficient iaa.intel.com/wq-user-dedicated, 1 Insufficient iaa.intel.com/wq-user-shared.
    ```
