# Intel GPU Device Plugin for Kubernetes

## QuickStart: build and run plugin on the host

### Prerequisites

- Computer with supported Intel GPU device running Linux
- Fully configured kubernetes node joined to the cluster
- Working [Go environment]
- Read access to the [Intel device plugins] git repository

### Get source code
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Build device plugin
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make
```

The result plugin executable is **cmd/gpu_plugin/gpu_plugin** 

### Configure kubelet

This instruction has been written with the assumption that the target
Kubernetes cluster is installed and configured with the `kubeadm` toolkit
from the official packages from http://kubernetes.io and the latest
tested version of Kubernetes is 1.9.

- Add DevicePlugins=true to the kubelet command line option --feature-gates

This can be done by creating a systemd drop-in for kubelet service in /etc/systemd/system/kubelet.service.d/
with the following content:
```
[Service]
Environment="KUBELET_EXTRA_ARGS=--feature-gates='DevicePlugins=true,HugePages=true'"
```

Check the man page for `systemd.unit` for more details on systemd drop-ins.

- Restart kubelet service with the new options
```
$ systemctl restart kubelet
```

- Make sure kubelet socket exists in /var/lib/kubelet/device-plugins/
```
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

### Run the plugin as administrator
```
$ sudo $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/gpu_plugin/gpu_plugin
GPU device plugin started
Adding '/dev/dri/card0' to GPU 'card0'
Adding '/dev/dri/controlD64' to GPU 'card0'
Adding '/dev/dri/renderD128' to GPU 'card0'
device-plugin start server at: /var/lib/kubelet/device-plugins/intelGPU.sock
device-plugin registered
device-plugin: ListAndWatch start
ListAndWatch: send devices &ListAndWatchResponse{Devices:[&Device{ID:card0,Health:Healthy,}],}
```

### Check if the plugin is registered on master
```
$ kubectl describe node <node name> | grep intel.com/gpu
 intel.com/gpu:  1
 intel.com/gpu:  1
```

There are more sophisticated ways to run device plugins. Please, consider reading [Device plugin deployment] to understand how to do it.

### Testing

1. Build a Docker image with beignet unit tests:

   ```
   $ cd demo
   $ ./build-image.sh ubuntu-demo-opencl
   ```

   This command will produce a Docker image named `ubuntu-demo-opencl`.

2. Create a pod running unit tests off the local Docker image:
   ```
   $ kubectl apply -f demo/intelgpu_pod.yaml
   ```

3. Observe the pod's logs:
   ```
   $ kubectl logs intelgpu-demo-pod
   ```

[Go environment]: https://golang.org/doc/install
[Intel device plugins]: https://github.com/intel/intel-device-plugins-for-kubernetes
[Device plugin deployment]: https://kubernetes.io/docs/concepts/cluster-administration/device-plugins/#device-plugin-deployment
