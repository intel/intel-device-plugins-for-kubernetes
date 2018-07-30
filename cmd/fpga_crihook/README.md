# Build and set up Intel FPGA prestart CRI-O hook

### Dependencies

You must install and set up the following FPGA plugin modules for correct operation:

-   [FPGA device plugin](cmd/fpga_plugin/README.md)
-   [FPGA admission controller webhook](cmd/fpga_admissionwebhook/README.md)
-   [FPGA prestart CRI-O hook](cmd/fpga_crihook/README.md) (this module)

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

### Install CRI-O hook:
```
$ sudo cp cmd/fpga_crihook/fpga_crihook /usr/local/bin/
```

### Configure CRI-O to run the hook:
```
$ sudo cat << EOF > /etc/containers/oci/hooks.d/prestart.json
{
    "hook" : "/usr/local/bin/fpga_crihook",
    "stage" : [ "prestart" ],
    "annotation": [ "intel.com/fpga-region" ]
}
EOF

$ sudo systemctl restart crio
```
