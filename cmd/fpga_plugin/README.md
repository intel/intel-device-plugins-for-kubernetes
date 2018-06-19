# Build and test Intel GPU Device Plugin for Kubernetes

### Get source code
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Build FPGA device plugin
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make fpga_plugin
```

### Make sure kubelet socket exists in /var/lib/kubelet/device-plugins/
```
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

#### Run FPGA device plugin in afu mode

##### Run FPGA device plugin as administrator
```
$ export NODE_NAME="<node name>" # if the node's name was overridden and differs from hostname
$ sudo -E $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/fpga_plugin -mode af -kubeconfig /var/run/kubernetes/admin.kubeconfig
FPGA device plugin started in af mode
device-plugin start server at: /var/lib/kubelet/device-plugins/intel-fpga-af-f7df405cbd7acf7222f144b0b93acd18.sock
device-plugin registered
```

##### Check if FPGA device plugin is registered on master
```
$ kubectl describe node <node name> | grep intel.com/fpga
 intel.com/fpga-af-f7df405cbd7acf7222f144b0b93acd18:  1
 intel.com/fpga-af-f7df405cbd7acf7222f144b0b93acd18:  1
```

#### Run FPGA device plugin in region mode

##### Run FPGA device plugin as administrator
```
$ export NODE_NAME="<node name>" # if the node's name was overridden and differs from hostname
$ sudo -E $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/fpga_plugin -mode region -kubeconfig /var/run/kubernetes/admin.kubeconfig
FPGA device plugin started in region mode
device-plugin start server at: /var/lib/kubelet/device-plugins/intel-fpga-region-ce48969398f05f33946d560708be108a.sock
device-plugin registered
```

##### Check if FPGA device plugin is registered on master
```
$ kubectl describe node <node name> | grep intel.com/fpga
 intel.com/fpga-region-ce48969398f05f33946d560708be108a:  1
 intel.com/fpga-region-ce48969398f05f33946d560708be108a:  1
```

### Deploy FPGA device plugin as DaemonSet

To deploy the plugin in a production cluster create a service account
for the plugin:

    $ kubectl create -f deployments/fpga_plugin/fpga_plugin_service_account.yaml
    serviceaccount/intel-fpga-plugin-controller created
    clusterrole.rbac.authorization.k8s.io/node-getter created
    clusterrolebinding.rbac.authorization.k8s.io/get-nodes created

Then create the DaemonSet itself

    $ kubectl create -f deployments/fpga_plugin/fpga_plugin.yaml
    daemonset.apps/intel-fpga-plugin created

You may want to modify the file `deployments/fpga_plugin/fpga_plugin.yaml` to
use your own container image. But the command

    $ make intel-fpga-plugin

can provide an image built from the sources. This image launches `fpga_plugin`
in the `af` mode by default. The mode can be overriden on per node basis with
this node annotation:

    $ kubectl annotate node mynode "fpga.intel.com/device-plugin-mode=region"
