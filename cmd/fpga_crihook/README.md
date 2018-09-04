# Build and set up Intel FPGA prestart CRI-O hook

### Dependencies

You must install and set up the following FPGA plugin modules for correct operation:

-   [FPGA device plugin](../fpga_plugin/README.md)
-   [FPGA admission controller webhook](../fpga_admissionwebhook/README.md)
-   [FPGA prestart CRI-O hook](README.md) (this module)

### Get source code:
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Build CRI-O hook:
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make fpga_crihook
```

### Download 'Acceleration Stack for Runtime' tarball
```
Download a10_gx_pac_ias_1_1_pv_rte_installer.tar.gz from https://www.intel.com/content/www/us/en/programmable/solutions/acceleration-hub/downloads.html into $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/deployments/fpga_plugin directory
```

### Build init container that contains CRI hook and all its dependencies:
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes/deployments/fpga_plugin
$ ./build-initcontainer-image.sh
```

