# Build and setup prestart CRI-O hook

### Get source code
```
$ mkdir -p $GOPATH/src/github.com/intel/
$ cd $GOPATH/src/github.com/intel/
$ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Build CRI-O hook
```
$ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
$ make fpga_crihook
```

### Install CRI-O hook
```
$ sudo cp cmd/fpga_crihook/fpga_crihook /usr/local/bin/
```

### Configure CRI-O to run the hook
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
