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

### Ensure that CRI-O is configured to allow OCI hooks

Recent versions of CRI-O are shipped with default configuration file that prevents
CRI-O to discover and configure hooks automatically.
For FPGA orchestration programmed mode, the OCI hooks are the key component.
Thus, please make sure that in your `/etc/crio/crio.conf` parameter `hooks_dir` is either unset (to enable default search paths for OCI hooks configuration) or contains directory `/etc/containers/oci/hooks.d`
