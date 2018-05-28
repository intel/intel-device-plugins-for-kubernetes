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
$ sudo $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/fpga_plugin -mode af
FPGA device plugin started in af mode
device-plugin start server at: /var/lib/kubelet/device-plugins/intel-fpga-f7df405cbd7acf7222f144b0b93acd18.sock
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
$ sudo $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/fpga_plugin/fpga_plugin -mode region
FPGA device plugin started in region mode
device-plugin start server at: /var/lib/kubelet/device-plugins/intel-fpga-ce48969398f05f33946d560708be108a.sock
device-plugin registered
```

##### Check if FPGA device plugin is registered on master
```
$ kubectl describe node <node name> | grep intel.com/fpga
 intel.com/fpga-region-ce48969398f05f33946d560708be108a:  1
 intel.com/fpga-region-ce48969398f05f33946d560708be108a:  1
```

