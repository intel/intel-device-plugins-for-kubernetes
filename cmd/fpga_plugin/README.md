# Build and test Intel FPGA device plugin for Kubernetes

### Dependencies

You must install and set up the following FPGA plugin modules for correct operation:

-   [FPGA device plugin](README.md) (this module)
-   [FPGA admission controller webhook](../fpga_admissionwebhook/README.md)
-   [FPGA prestart CRI-O hook](../fpga_crihook/README.md)


### Get source code:
```
    $ mkdir -p $GOPATH/src/github.com/intel/
    $ cd $GOPATH/src/github.com/intel/
    $ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Build FPGA device plugin:
```
    $ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
    $ make fpga_plugin
```

### Verify kubelet socket exists in /var/lib/kubelet/device-plugins/ directory:
```
    $ ls /var/lib/kubelet/device-plugins/kubelet.sock
    /var/lib/kubelet/device-plugins/kubelet.sock
```

### Choose mode for FPGA device plugin

You can run the FPGA device plugin in either `af` or `region` mode.

In `af` mode, the device plugin exposes accelerator functions
programmed onto the device as resources. Accelerator functions identified
by the same AFU ID are translated to resources of the same type.

In `region` mode, the device plugin exposes partial reconfiguration regions
as consumable resources. Regions with the same interface ID are
translated to resources of the same type.

### Deploy FPGA device plugin as host process for development purposes

#### Run FPGA device plugin in af mode

1. Run FPGA device plugin as administrator:
```
    $ export KUBE_CONF=/var/run/kubernetes/admin.kubeconfig # path to kubeconfig with admin's credentials
    $ export NODE_NAME="<node name>" # if the node's name was overridden and differs from hostname
    $ sudo -E $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/fpga_plugin -mode af -kubeconfig $KUBE_CONF
    FPGA device plugin started in af mode
    device-plugin start server at: /var/lib/kubelet/device-plugins/fpga.intel.com-af-f7df405cbd7acf7222f144b0b93acd18.sock
    device-plugin registered
```

**Note**: It is also possible to run the FPGA device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to device plugin socket creation and kubelet registration.
Furthermore, the deployments `securityContext` must be configured with appropriate `runAsUser/runAsGroup`.

2. Check if FPGA device plugin is registered on master:
```
    $ kubectl describe node <node name> | grep fpga.intel.com
    fpga.intel.com/af-f7df405cbd7acf7222f144b0b93acd18:  1
    fpga.intel.com/af-f7df405cbd7acf7222f144b0b93acd18:  1
```

#### Run FPGA device plugin in region mode

1. Run FPGA device plugin as administrator:
```
    $ export KUBE_CONF=/var/run/kubernetes/admin.kubeconfig # path to kubeconfig with admin's credentials
    $ export NODE_NAME="<node name>" # if the node's name was overridden and differs from hostname
    $ sudo -E $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/fpga_plugin -mode region -kubeconfig $KUBE_CONF
    FPGA device plugin started in region mode
    device-plugin start server at: /var/lib/kubelet/device-plugins/fpga.intel.com-region-ce48969398f05f33946d560708be108a.sock
    device-plugin registered
```

2. Check if FPGA device plugin is registered on master:
```
    $ kubectl describe node <node name> | grep fpga.intel.com
    fpga.intel.com/region-ce48969398f05f33946d560708be108a:  1
    fpga.intel.com/region-ce48969398f05f33946d560708be108a:  1
```

### Deploy FPGA device plugin as DaemonSet

1. To deploy the plugin in a production cluster, create a service account
for the plugin:
    ```
        $ kubectl create -f deployments/fpga_plugin/fpga_plugin_service_account.yaml
        serviceaccount/intel-fpga-plugin-controller created
        clusterrole.rbac.authorization.k8s.io/node-getter created
        clusterrolebinding.rbac.authorization.k8s.io/get-nodes created
    ```

2. Create the DaemonSet:
    ```
        $ kubectl create -f deployments/fpga_plugin/fpga_plugin.yaml
        daemonset.apps/intel-fpga-plugin created
    ```

3. Build an image from sources:
    ```
        $ make intel-fpga-plugin
    ```
    This image launches `fpga_plugin` in `af` mode by default.

    You can override the mode on a per-node basis using this annotation:
    ```
        $ kubectl annotate node mynode "fpga.intel.com/device-plugin-mode=region"
    ```
    To use your own container image, modify the
    `deployments/fpga_plugin/fpga_plugin.yaml` file.

### Next steps

Continue with [FPGA admission controller webhook](../fpga_admissionwebhook/README.md).

