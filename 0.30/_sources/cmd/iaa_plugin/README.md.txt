# Intel IAA device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Pre-built images](#pre-built-images)
    * [Verify plugin registration](#verify-plugin-registration)
* [Testing and Demos](#testing-and-demos)

## Introduction

The IAA device plugin for Kubernetes supports acceleration using the Intel Analytics accelerator(IAA).

The IAA plugin discovers IAA work queues and presents them as a node resources.

The IAA plugin and operator optionally support provisioning of IAA devices and workqueues with the help of [accel-config](https://github.com/intel/idxd-config) utility through initcontainer.

## Installation

The following sections detail how to use the IAA device plugin.

### Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-iaa-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/iaa_plugin?ref=<RELEASE_VERSION>'
daemonset.apps/intel-iaa-plugin created
```

Where `<RELEASE_VERSION>` needs to be substituted with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

Nothing else is needed. See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the plugin.

#### Automatic Provisioning

There's a sample [idxd initcontainer](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/build/docker/intel-idxd-config-initcontainer.Dockerfile) included that provisions IAA devices and workqueues (1 engine / 1 group / 1 wq (user/dedicated)), to deploy:

```bash
$ kubectl apply -k deployments/iaa_plugin/overlays/iaa_initcontainer/
```

The provisioning [script](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/idxd-init.sh) and [template](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/iaa.conf) are available for customization.

The provisioning config can be optionally stored in the ProvisioningConfig configMap which is then passed to initcontainer through the volume mount.

There's also a possibility for a node specific congfiguration through passing a nodename via NODE_NAME into initcontainer's environment and passing a node specific profile via configMap volume mount.

To create a custom provisioning config:

```bash
$ kubectl create configmap --namespace=inteldeviceplugins-system intel-iaa-config --from-file=demo/iaa.conf
```

### Verify Plugin Registration

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

## Testing and Demos

We can test the plugin is working by deploying the provided example accel-config-demo test image.

1. Build a Docker image with an accel-config tests:

    ```bash
    $ make accel-config-demo
    ...
    Successfully tagged intel/accel-config-demo:devel
    ```

1. Create a pod running unit tests off the local Docker image:

    ```bash
    $ kubectl apply -f ./demo/iaa-accel-config-demo-pod.yaml
    pod/iaa-accel-config-demo created
    ```

1. Wait until pod is completed:

    ```bash
    $ kubectl get pods  |grep iaa-accel-config-demo
    iaa-accel-config-demo    0/1     Completed   0          31m

    If the pod did not successfully launch, possibly because it could not obtain the IAA
    resource, it will be stuck in the `Pending` status:

    ```bash
    $ kubectl get pods
    NAME                      READY   STATUS    RESTARTS   AGE
    iaa-accel-config-demo     0/1     Pending   0          7s
    ```

    This can be verified by checking the Events of the pod:

    ```bash

    $ kubectl describe pod iaa-accel-config-demo | grep -A3 Events:
    Events:
      Type     Reason            Age    From               Message
      ----     ------            ----   ----               -------
      Warning  FailedScheduling  2m26s  default-scheduler  0/1 nodes are available: 1 Insufficient iaa.intel.com/wq-user-dedicated.
    ```
