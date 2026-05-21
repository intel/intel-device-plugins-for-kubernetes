# Intel DSA device plugin for Kubernetes

Table of Contents

* [Introduction](#introduction)
* [Installation](#installation)
    * [Pre-built Images](#pre-built-images)
    * [Verify Plugin Registration](#verify-plugin-registration)
* [Testing and Demos](#testing-and-demos)

## Introduction

The DSA device plugin for Kubernetes supports acceleration using the Intel Data Streaming accelerator(DSA).

The DSA plugin discovers DSA work queues and presents them as a node resources.

The DSA plugin and operator optionally support provisioning of DSA devices and workqueues with the help of [accel-config](https://github.com/intel/idxd-config) utility through initcontainer.

## Installation

The following sections detail how to use the DSA device plugin.

### Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-dsa-plugin)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository. Thus the easiest way to deploy the plugin in your cluster is to run this command

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/dsa_plugin?ref=<RELEASE_VERSION>'
daemonset.apps/intel-dsa-plugin created
```

Where `<RELEASE_VERSION>` needs to be substituted with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

Nothing else is needed. See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the plugin.

#### Automatic Provisioning

There's a sample [idxd initcontainer](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/build/docker/intel-idxd-config-initcontainer.Dockerfile) included that provisions DSA devices and workqueues (1 engine / 1 group / 1 wq (user/dedicated)), to deploy:

```bash
$ kubectl apply -k deployments/dsa_plugin/overlays/dsa_initcontainer/
```

The provisioning [script](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/main/demo/idxd-init.sh) and [template](https://github.com/intel/intel-device-plugins-for-kubernetes/blob/master/demo/dsa.conf) are available for customization.

The provisioning config can be optionally stored in the ProvisioningConfig configMap which is then passed to initcontainer through the volume mount.

There's also a possibility for a node specific congfiguration through passing a nodename via NODE_NAME into initcontainer's environment and passing a node specific profile via configMap volume mount.

To create a custom provisioning config:

```bash
$ kubectl create configmap --namespace=inteldeviceplugins-system intel-dsa-config --from-file=demo/dsa.conf
```

### Verify Plugin Registration
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

## Testing and Demos

We can test the plugin is working by deploying the provided example accel-config test image.

1. Build a Docker image with an accel-config tests:

    ```bash
    $ make accel-config-demo
    ...
    Successfully tagged accel-config-demo:devel
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
