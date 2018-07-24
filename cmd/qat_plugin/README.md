# Build and test Intel QuickAssist (QAT) Device Plugin for Kubernetes

### Requirements/Prerequisites
* Ensure that DPDK drivers are loaded and ready to be used. Please refer to the following for more information on drivers:
  * [DPDK Getting Started Guide for Linux](https://doc.dpdk.org/guides/linux_gsg/index.html)
  * [Linux Drivers](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html)
* Ensure that the QuickAssist SR-IOV virtual functions are configured. Verify this by running:
```
lspci | grep QAT
```
 For installation and configuring of Intel QuickAssist Technology software for Linux refer to the [Intel QuickAssist Technology Software for Linux - Getting Started Guide] (https://01.org/sites/default/files/downloads/intelr-quickassist-technology/336212qatswgettingstartedguiderev003.pdf) on [ https://01.org/intel-quickassist-technology](https://01.org/intel-quickassist-technology)

### Get source code
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```
### Make sure kubelet socket exists in /var/lib/kubelet/device-plugins/
```
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```
### Build QAT device plugin
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make qat_plugin
```
### Deploy QAT device plugin directly on the host
```
$ sudo $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/cmd/qat_plugin/qat_plugin \
-dpdk-driver igb_uio -kernel-vf-drivers dh895xccvf -max-num-devices 10 -v 10 logtostderr
QAT device plugin started
Discovered Devices below:
03:01.0 device: corresponding DPDK device detected is uio0
03:01.1 device: corresponding DPDK device detected is uio1
03:01.2 device: corresponding DPDK device detected is uio2
03:01.3 device: corresponding DPDK device detected is uio3
03:01.4 device: corresponding DPDK device detected is uio4
03:01.5 device: corresponding DPDK device detected is uio5
03:01.6 device: corresponding DPDK device detected is uio6
03:01.7 device: corresponding DPDK device detected is uio7
03:02.0 device: corresponding DPDK device detected is uio8
03:02.1 device: corresponding DPDK device detected is uio9
The number of devices discovered are:10
device-plugin start server at: /var/lib/kubelet/device-plugins/intelQAT.sock
device-plugin registered
ListAndWatch: Sending device response

```
* By default the device plugin supports QuickAssist devices DH895xCC,C62x,C3xxx and D15xx devices. The kernel-vf-drivers flag can be used to specify the vf Device Driver of QAT device type. For more information please refer [here](https://dpdk.org/doc/guides/cryptodevs/qat.html).

### Build QAT device plugin docker image
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make  intel-qat-plugin
```
### Deploy QAT device plugin as a DaemonSet
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
kubectl create -f deployments/qat_plugin/qat_plugin.yaml
```
### Check if QAT device plugin is registered on master
```
$ kubectl describe node <node name> | grep intel.com/qat
 intel.com/qat: 10
 intel.com/qat: 10
```

### Consuming QAT device plugin

1. Place the Dockerfile in the DPDK directory and build the DPDK image:

   ```
   $ cd demo
   $ docker build -t crypto-perf .
   ```

   This command will produce a Docker image named `ubuntu-demo-opencl`.

2. Running an example DPDK application (dpdk-test-crypto-perf) requesting QAT devices:

   Add container resource request and limit e.g. **intel.com/qat: '<number of devices>'** for the container requesting quick assist device in the pod specification file. For a DPDK based workload hugepage request and limit might also be needed.
   ```
   $ kubectl create -f demo/crypto-perf-dpdk-pod-requesting-qat.yaml
   $ kubectl get pods
     NAME                     READY     STATUS    RESTARTS   AGE
     dpdkqatuio               1/1       Running   0          27m
     intel-qat-plugin-5zgvb   1/1       Running   0          3h


   $ kubectl exec -it dpdkqatuio bash

   ```

3. Execute the dpdk-test-crypto-perf application and see the logs:
   ```
   $ ./dpdk-test-crypto-perf -l 6-7 -w $intelQAT1 -- --ptest throughput --\
	devtype crypto_qat --optype cipher-only --cipher-algo aes-cbc --cipher-op \
	encrypt --cipher-key-sz 16 --total-ops 10000000 --burst-sz 32 --buffer-sz 64

   ```
