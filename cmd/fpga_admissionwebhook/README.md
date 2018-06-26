## Build and install Intel FPGA webhook for admission controller

### Get source code

    $ mkdir -p $GOPATH/src/github.com/intel/
    $ cd $GOPATH/src/github.com/intel/
    $ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git

### Build a Docker image with the webhook

    $ export SRC=$GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
    $ cd $SRC
    $ make intel-fpga-admissionwebhook
    $ docker images
    REPOSITORY                    TAG                                        IMAGE ID            CREATED          SIZE
    intel-fpga-admissionwebhook   10efe163a5091e8b2ceaa9baad236d3a41063c88   6c3bce0b8693        0 sec ago        25.2MB
    intel-fpga-admissionwebhook   devel                                      6c3bce0b8693        0 sec ago        25.2MB
    ...

### Deploy webhook service

Make sure you have `cfssl` and `jq` utilities installed on your host.
Then run the script `scripts/webhook-deploy.sh`.

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

By default the script deploys the webhook in the preprogrammed mode (when
requested FPGA resources get translated to AF resources, e.g.
"intel.com/fpga-arria10-nlb0" -> "intel.com/fpga-af-d8424dc4a4a3c413f89e433683f9040b").
You can command the script to deploy the webhook in the orchestrated mode with
the option `--mode`.

    $ ./scripts/webhook-deploy.sh --mode orchestrated

Please note that the script needs the CA bundle used for signing cerificate
requests in your cluster. By default it fetches the bundle stored
in the configmap `extension-apiserver-authentication`. But it may differ from
the actual signing cerificate which is passed in the option
`--cluster-signing-cert-file` to `kube-controller-manager`. In this case
you need to point the script to the actual signing cerificate:

    $ ./scripts/webhook-deploy.sh --ca-bundle-path /var/run/kubernetes/server-ca.crt
