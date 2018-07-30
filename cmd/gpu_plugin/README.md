# Build and test Intel GPU device plugin for Kubernetes

### Get source code: ZZZ H3 *(as it was)*
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

#### Build GPU device plugin:  ZZZ H4 my **change**
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make gpu_plugin
```

#### Verify kubelet socket exists in /var/lib/kubelet/device-plugins/ directory:
```
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

#### Run GPU device plugin as administrator:
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

#### Verify GPU device plugin is registered on master:
```
$ kubectl describe node <node name> | grep intel.com/gpu
 intel.com/gpu:  1
 intel.com/gpu:  1
```

#### Test GPU device plugin:

1. Build a Docker image with beignet unit tests:
   ```
   $ cd demo
   $ ./build-image.sh ubuntu-demo-opencl
   ```

      This command produces a Docker image named `ubuntu-demo-opencl`.

2. Create a pod running unit tests off the local Docker image:
   ```
   $ kubectl apply -f demo/intelgpu_job.yaml
   ```

3. Observe the pod's logs:
   ```
   $ kubectl logs intelgpu-demo-job-xxxx
   ```
