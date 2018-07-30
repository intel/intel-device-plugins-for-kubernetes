# Build and install Intel FPGA webhook for admission controller

### Dependencies

You must install and set up the following FPGA plugin modules for correct operation:

-   [FPGA device plugin](cmd/fpga_plugin/README.md)
-   [FPGA admission controller webhook](cmd/fpga_admissionwebhook/README.md) (this module)
-   [FPGA prestart CRI-O hook](cmd/fpga_crihook/README.md)

### Get source code:
```
    $ mkdir -p $GOPATH/src/github.com/intel/
    $ cd $GOPATH/src/github.com/intel/
    $ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git
```

### Build a Docker image with the webhook:
```
    $ export SRC=$GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
    $ cd $SRC
    $ make intel-fpga-admissionwebhook
    $ docker images
    REPOSITORY                    TAG                                        IMAGE ID            CREATED          SIZE
    intel-fpga-admissionwebhook   10efe163a5091e8b2ceaa9baad236d3a41063c88   6c3bce0b8693        0 sec ago        25.2MB
    intel-fpga-admissionwebhook   devel                                      6c3bce0b8693        0 sec ago        25.2MB
    ...
```

### Deploy webhook service:

Verify that the `cfssl` and `jq` utilities are installed on your host.
Run the `scripts/webhook-deploy.sh` script.
```
    $ cd $SRC
    $ ./scripts/webhook-deploy.sh
    Create secret including signed key/cert pair for the webhook
    Creating certs in /tmp/tmp.JYgcFiaoCZ
    certificatesigningrequest "intel-fpga-webhook-svc.default" created
    NAME                             AGE       REQUESTOR      CONDITION
    intel-fpga-webhook-svc.default   1s        system:admin   Pending
    certificatesigningrequest "intel-fpga-webhook-svc.default" approved
    secret "intel-fpga-webhook-certs" created
    Removing /tmp/tmp.JYgcFiaoCZ
    Create webhook deployment
    deployment "intel-fpga-webhook-deployment" created
    Create webhook service
    service "intel-fpga-webhook-svc" created
    Register webhook
    mutatingwebhookconfiguration "fpga-mutator-webhook-cfg" created
```

By default, the script deploys the webhook in a preprogrammed mode. Requested FPGA resources are translated to AF resources. For example, 
`intel.com/fpga-arria10-nlb0` is translated to `intel.com/fpga-af-d8424dc4a4a3c413f89e433683f9040b`.

Use the option `--mode` to command the script to deploy the webhook in orchestrated mode:
```
    $ ./scripts/webhook-deploy.sh --mode orchestrated
```

Note that the script needs the CA bundle used for signing certificate
requests in your cluster. By default, the script fetches the bundle stored
in the configmap `extension-apiserver-authentication`. However, your cluster may use a different signing certificate that is passed in the option
`--cluster-signing-cert-file` to `kube-controller-manager`. In this case,
you must point the script to the actual signing certificate as follows:
```
    $ ./scripts/webhook-deploy.sh --ca-bundle-path /var/run/kubernetes/server-ca.crt
```

### Next steps

Continue with [FPGA prestart CRI-O hook](cmd/fpga_crihook/README.md).
