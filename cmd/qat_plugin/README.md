# Build and test Intel® QuickAssist Technology (QAT) device plugin for Kubernetes

### Prerequisites
* Data Plane Development Kit (DPDK) drivers must be loaded and configured. For more information, refer to:
    * [DPDK Getting Started Guide for Linux](https://doc.dpdk.org/guides/linux_gsg/index.html)
    * [DPDK Getting Started Guide, Linux Drivers section](http://dpdk.org/doc/guides/linux_gsg/linux_drivers.html)
* QuickAssist SR-IOV virtual functions must be configured. Verify this by running:
```
lspci | grep QAT
```
* Intel QuickAssist Technology software for Linux must be installed and
  configured. For more information, refer to:
    * [Intel QuickAssist Technology Software for Linux - Getting Started Guide](https://01.org/sites/default/files/downloads/intelr-quickassist-technology/336212qatswgettingstartedguiderev003.pdf)
    * [Intel QuickAssist Technology on 01.org](https://01.org/intel-quickassist-technology)

### Get source code:
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Verify kubelet socket exists in /var/lib/kubelet/device-plugins/ directory:
```
$ ls /var/lib/kubelet/device-plugins/kubelet.sock
/var/lib/kubelet/device-plugins/kubelet.sock
```

### Build QAT device plugin:
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make qat_plugin
```

### Deploy QAT device plugin directly on the host:
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

By default, the device plugin supports these QuickAssist devices:  DH895xCC, C62x, C3xxx, and D15xx devices.

Use the `kernel-vf-drivers flag` to specify the vf Device Driver for the particular QAT device. For more information, refer to [Intel QAT Crypto Poll Mode Driver](https://dpdk.org/doc/guides/cryptodevs/qat.html).

### Build QAT device plugin Docker image:
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make  intel-qat-plugin
```

### Deploy QAT device plugin as a DaemonSet:
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
kubectl create -f deployments/qat_plugin/qat_plugin.yaml
```

### Verify QAT device plugin is registered on master:
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

     This command produces a Docker image named `ubuntu-demo-opencl`.

2. Run an example DPDK application (`dpdk-test-crypto-perf`) requesting QAT devices:

      In the pod specification file, add container resource request and limit.
      For example, `intel.com/qat: <number of devices>` for a container requesting QAT devices.

      For a DPDK based workload, you may need to add hugepage request and limit.

     ```
     $ kubectl create -f demo/crypto-perf-dpdk-pod-requesting-qat.yaml
     $ kubectl get pods
       NAME                     READY     STATUS    RESTARTS   AGE
       dpdkqatuio               1/1       Running   0          27m
       intel-qat-plugin-5zgvb   1/1       Running   0          3h

     $ kubectl exec -it dpdkqatuio bash
     ```

3. Execute the `dpdk-test-crypto-perf` application and review the logs:
   ```
   $ ./dpdk-test-crypto-perf -l 6-7 -w $intelQAT1 -- --ptest throughput --\
	devtype crypto_qat --optype cipher-only --cipher-algo aes-cbc --cipher-op \
	encrypt --cipher-key-sz 16 --total-ops 10000000 --burst-sz 32 --buffer-sz 64

   ```
