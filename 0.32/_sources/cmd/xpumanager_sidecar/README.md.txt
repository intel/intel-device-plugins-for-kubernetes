# XeLink sidecar for Intel XPU Manager

Table of Contents

* [Introduction](#introduction)
* [Modes and Configuration Options](#modes-and-configuration-options)
* [Installation](#installation)
  * [Install XPU Manager with the Sidecar](#install-xpu-manager-with-the-sidecar)
  * [Install Sidecar to an Existing XPU Manager](#install-sidecar-to-an-existing-xpu-manager)
* [Verify Sidecar Functionality](#verify-sidecar-functionality)
* [Use HTTPS with XPU Manager](#use-https-with-xpu-manager)

## Introduction

Intel GPUs can be interconnected via an XeLink. In some workloads it is beneficial to use GPUs that are XeLinked together for optimal performance. XeLink information is provided by [Intel XPU Manager](https://www.github.com/intel/xpumanager) via its metrics API. Xelink sidecar retrieves the information from XPU Manager and stores it on the node under ```/etc/kubernetes/node-feature-discovery/features.d/``` as a feature label file. [NFD](https://github.com/kubernetes-sigs/node-feature-discovery) reads this file and converts it to Kubernetes node labels. These labels are then used by [GAS](https://github.com/intel/platform-aware-scheduling/tree/master/gpu-aware-scheduling) to make [scheduling decisions](https://github.com/intel/platform-aware-scheduling/blob/master/gpu-aware-scheduling/docs/usage.md#multi-gpu-allocation-with-xe-link-connections) for Pods.

## Modes and Configuration Options

| Flag | Argument | Default | Meaning |
|:---- |:-------- |:------- |:------- |
| -lane-count | int | 4 | Minimum lane count for an XeLink interconnect to be accepted |
| -interval | int | 10 | Interval for XeLink topology fetching and label writing (seconds, >= 1) |
| -startup-delay | int | 10 | Startup delay before the first topology fetching (seconds, >= 0) |
| -label-namespace | string | gpu.intel.com | Namespace or prefix for the labels. i.e. **gpu.intel.com**/xe-links |
| -allow-subdeviceless-links | bool | false | Include xelinks also for devices that do not have subdevices |
| -cert | string | "" | Use HTTPS and verify server's endpoint |

The sidecar also accepts a number of other arguments. Please use the -h option to see the complete list of options.

## Installation

The following sections detail how to obtain, deploy and test the XPU Manager XeLink sidecar.

### Pre-built Images

[Pre-built images](https://hub.docker.com/r/intel/intel-xpumanager-sidecar)
of this component are available on the Docker hub. These images are automatically built and uploaded
to the hub from the latest main branch of this repository.

Release tagged images of the components are also available on the Docker hub, tagged with their
release version numbers in the format `x.y.z`, corresponding to the branches and releases in this
repository.

Note: Replace `<RELEASE_VERSION>` with the desired [release tag](https://github.com/intel/intel-device-plugins-for-kubernetes/tags) or `main` to get `devel` images.

See [the development guide](../../DEVEL.md) for details if you want to deploy a customized version of the plugin.

#### Install XPU Manager with the Sidecar

Install XPU Manager daemonset with the XeLink sidecar

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/xpumanager_sidecar/overlays/http?ref=<RELEASE_VERSION>'
```

Please see XPU Manager Kubernetes files for additional info on [installation](https://github.com/intel/xpumanager/tree/master/deployment/kubernetes).

#### Install Sidecar to an Existing XPU Manager

Use patch to add sidecar into the XPU Manager daemonset.

```bash
$ kubectl patch daemonsets.apps intel-xpumanager --patch-file 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/xpumanager_sidecar/overlays/http/xpumanager.yaml?ref=<RELEASE_VERSION>'
```

NOTE: The sidecar patch will remove other resources from the XPU Manager container. If your XPU Manager daemonset is using, for example, the smarter device manager resources, those will be removed.

### Verify Sidecar Functionality

You can verify the sidecar's functionality by checking node's xe-links labels:

```bash
$ kubectl get nodes -A -o=jsonpath="{range .items[*]}{.metadata.name},{.metadata.labels.gpu\.intel\.com\/xe-links}{'\n'}{end}"
master,0.0-1.0_0.1-1.1
```

### Use HTTPS with XPU Manager

There is an alternative deployment that uses HTTPS instead of HTTP. The reference deployment requires `cert-manager` to provide a certificate for TLS. To deploy:

```bash
$ kubectl apply -k 'https://github.com/intel/intel-device-plugins-for-kubernetes/deployments/xpumanager_sidecar/overlays/cert-manager?ref=<RELEASE_VERSION>'
```

The deployment requests a certificate and key from `cert-manager`. They are then provided to the gunicorn container as secrets and are used in the HTTPS interface. The sidecar container uses the same certificate to verify the server.

> *NOTE*: The HTTPS deployment uses self-signed certificates. For production use, the certificates should be properly set up.

<details>
<summary>Enabling HTTPS manually</summary>

If one doesn't want to use `cert-manager`, the same can be achieved manually by creating certificates with openssl and then adding it to the deployment. The steps are roughly:
1) Create a certificate with [openssl](https://www.linode.com/docs/guides/create-a-self-signed-tls-certificate/)
1) Create a secret from the [certificate & key](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_create/kubectl_create_secret_tls/).
1) Change the deployment:

* Add certificate and key to gunicorn container:
```
      - command:
        - gunicorn
...
        - --certfile=/certs/tls.crt
        - --keyfile=/certs/tls.key
...
        - xpum_rest_main:main()
```

* Add secret mounting to the Pod:
```
    containers:
      - name: python-exporter
        volumeMounts:
        - mountPath: /certs
          name: certs
          readOnly: true
    volumes:
    - name: certs
      secret:
        defaultMode: 420
        secretName: xpum-server-cert
 ```

* Add use-https and cert to sidecar
```
        name: xelink-sidecar
        volumeMounts:
        - mountPath: /certs
          name: certs
          readOnly: true
        args:
...
          - --cert=/certs/tls.crt
...
```

</details>
