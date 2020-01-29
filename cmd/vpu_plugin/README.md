# Build and test Intel VPU device plugin for Kubernetes

### Get source code:
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```
Note: to get VCAC-A card running hddl, please refer to:
https://github.com/OpenVisualCloud/Dockerfiles/blob/master/VCAC-A/script/setup_hddl.sh

### Verify kubelet socket exists in /var/lib/kubelet/device-plugins/ directory:
```
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

### Deploy VPU device plugin as host process for development purposes

#### Build VPU device plugin:
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make vpu_plugin
```

#### Run VPU device plugin as administrator:
```
$ sudo $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/vpu_plugin/vpu_plugin
device-plugin start server at: /var/lib/kubelet/device-plugins/vpu.intel.com-hddl.sock
device-plugin registered
```

### Deploy VPU device plugin as a DaemonSet:

#### Build plugin image
```
$ make intel-vpu-plugin
```

#### Create plugin DaemonSet
```
$ kubectl create -f ./deployments/vpu_plugin/vpu_plugin.yaml
daemonset.apps/intel-vpu-plugin created
```
**Note**: It is also possible to run the VPU device plugin using a non-root user. To do this,
the nodes' DAC rules must be configured to allow USB device descriptor detection, device plugin
socket creation and kubelet registration. Furthermore, the deployments `securityContext` must
be configured with appropriate `runAsUser/runAsGroup`.

### Verify VPU device plugin is registered on master:
```
$ kubectl describe node <node name> | grep vpu.intel.com
 vpu.intel.com/hddl:  1
 vpu.intel.com/hddl:  1
```

### Test VPU device plugin:

1. Build a Docker image with an example openvino to VPU:
   ```
   $ cd demo
   $ ./build-image.sh ubuntu-demo-openvino
   ```

      This command produces a Docker image named `ubuntu-demo-openvino`.

2. Create a pod running unit tests off the local Docker image:
   ```
   $ kubectl apply -f demo/intelvpu-job.yaml
   ```

3. Review the pod's logs:
   ```
   $ kubectl logs intelvpu-demo-job-xxxx
   ```
