# Build and install Intel FPGA webhook for admission controller

### Dependencies

You must install and set up the following FPGA plugin modules for correct operation:

-   [FPGA device plugin](../fpga_plugin/README.md)
-   [FPGA admission controller webhook](README.md) (this module)
-   [FPGA prestart CRI-O hook](../fpga_crihook/README.md)

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
    Creating certs in /tmp/tmp.Ebb77GBKqm
    certificatesigningrequest.certificates.k8s.io/intel-fpga-webhook-svc.default created
    NAME                             AGE       REQUESTOR      CONDITION
    intel-fpga-webhook-svc.default   0s        system:admin   Pending
    certificatesigningrequest.certificates.k8s.io/intel-fpga-webhook-svc.default approved
    secret/intel-fpga-webhook-certs created
    Removing /tmp/tmp.Ebb77GBKqm
    Create FPGA CRDs
    customresourcedefinition.apiextensions.k8s.io/acceleratorfunctions.fpga.intel.com created
    customresourcedefinition.apiextensions.k8s.io/fpgaregions.fpga.intel.com created
    acceleratorfunction.fpga.intel.com/arria10-nlb0 created
    acceleratorfunction.fpga.intel.com/arria10-nlb3 created
    fpgaregion.fpga.intel.com/arria10 created
    clusterrole.rbac.authorization.k8s.io/fpga-reader created
    clusterrolebinding.rbac.authorization.k8s.io/default-fpga-reader created
    Create webhook deployment
    deployment.extensions/intel-fpga-webhook-deployment created
    Create webhook service
    service/intel-fpga-webhook-svc created
    Register webhook
    mutatingwebhookconfiguration.admissionregistration.k8s.io/fpga-mutator-webhook-cfg created
```

Mappings of resource names are configured with objects of `AcceleratorFunction` and
`FpgaRegion` custom resource definitions found respectively in
`./deployment/fpga_admissionwebhook/af-crd.yaml` and `./deployment/fpga_admissionwebhook/region-crd.yaml`.

Please note that the mappings are scoped to the namespaces they were created in
and are applicable to pods created in corresponding namespaces.

By default, the script deploys the webhook in a preprogrammed mode. Requested FPGA resources are translated to AF resources. For example,
`fpga.intel.com/arria10-nlb0` is translated to `fpga.intel.com/af-d8424dc4a4a3c413f89e433683f9040b`.

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

Continue with [FPGA prestart CRI-O hook](../fpga_crihook/README.md).
