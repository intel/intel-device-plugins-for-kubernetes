## Build and install Intel FPGA webhook for admission controller

### Get source code

    $ mkdir -p $GOPATH/src/github.com/intel/
    $ cd $GOPATH/src/github.com/intel/
    $ git clone https://github.com/intel/intel-device-plugins-for-kubernetes.git

### Build a Docker image with the webhook

    $ cd $GOPATH/src/github.com/intel/intel-device-plugins-for-kubernetes
    $ make webhook-container
    $ docker images
    REPOSITORY                    TAG                                        IMAGE ID            CREATED          SIZE
    intel-fpga-admissionwebhook   878381826cdef0b112234c296d4e13d3266455ae   0fbbf9dfae95        0 sec ago        24.7MB
    ...

### Create secret including signed key/cert pair for the webhook

Use the following script taken from [this article](https://medium.com/ibm-cloud/diving-into-kubernetes-mutatingadmissionwebhook-6ef3c5695f74):

```bash
#!/bin/bash

while [[ $# -gt 0 ]]; do
    case ${1} in
        --service)
            service="$2"
            shift
            ;;
        --secret)
            secret="$2"
            shift
            ;;
        --namespace)
            namespace="$2"
            shift
            ;;
	--kubectl)
	    kubectl="$2"
            shift
            ;;
    esac
    shift
done

[ -z ${service} ] && service=intel-fpga-webhook-svc
[ -z ${secret} ] && secret=intel-fpga-webhook-certs
[ -z ${namespace} ] && namespace=default
[ -z ${kubectl} ] && kubectl=kubectl

csrName=${service}.${namespace}
tmpdir=$(mktemp -d)
echo "creating certs in tmpdir ${tmpdir} "

cat <<EOF >> ${tmpdir}/csr.conf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${service}
DNS.2 = ${service}.${namespace}
DNS.3 = ${service}.${namespace}.svc
EOF

openssl genrsa -out ${tmpdir}/server-key.pem 2048
openssl req -new -key ${tmpdir}/server-key.pem -subj "/CN=${service}.${namespace}.svc" -out ${tmpdir}/server.csr -config ${tmpdir}/csr.conf

# clean-up any previously created CSR for our service. Ignore errors if not present.
${kubectl} delete csr ${csrName} 2>/dev/null || true

# create  server cert/key CSR and  send to k8s API
cat <<EOF | ${kubectl} create -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${csrName}
spec:
  groups:
  - system:authenticated
  request: $(cat ${tmpdir}/server.csr | base64 -w 0)
  usages:
  - digital signature
  - key encipherment
  - server auth
EOF

# verify CSR has been created
while true; do
    ${kubectl} get csr ${csrName}
    if [ "$?" -eq 0 ]; then
        break
    fi
done

# approve and fetch the signed certificate
${kubectl} certificate approve ${csrName}

# verify certificate has been signed
for x in $(seq 10); do
    serverCert=$(${kubectl} get csr ${csrName} -o jsonpath='{.status.certificate}')
    if [[ ${serverCert} != '' ]]; then
        break
    fi
    sleep 1
done

if [[ ${serverCert} == '' ]]; then
    echo "ERROR: After approving csr ${csrName}, the signed certificate did not appear on the resource. Giving up after 10 attempts." >&2
    exit 1
fi

echo ${serverCert} | openssl base64 -d -A -out ${tmpdir}/server-cert.pem


# create the secret with CA cert and server cert/key
${kubectl} create secret generic ${secret} \
        --from-file=key.pem=${tmpdir}/server-key.pem \
        --from-file=cert.pem=${tmpdir}/server-cert.pem \
        --dry-run -o yaml |
    ${kubectl} -n ${namespace} apply -f -
```

You should see something like

    $ ./scripts/webhook-create-signed-cert.sh
    creating certs in tmpdir /tmp/tmp.9sgk16v5Y2 
    Generating RSA private key, 2048 bit long modulus
    .........................+++
    ....+++
    e is 65537 (0x010001)
    certificatesigningrequest "intel-fpga-webhook-svc.default" created
    NAME                             AGE       REQUESTOR      CONDITION
    intel-fpga-webhook-svc.default   0s        system:admin   Pending
    certificatesigningrequest "intel-fpga-webhook-svc.default" approved
    secret "intel-fpga-webhook-certs" created

### Deploy webhook service

Using the following specs:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
      name: intel-fpga-webhook-deployment
      labels:
        app: intel-fpga-webhook
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: intel-fpga-webhook
    spec:
      containers:
        - name: fpga-mutator
          image: intel-fpga-admissionwebhook:878381826cdef0b112234c296d4e13d3266455ae
          imagePullPolicy: IfNotPresent
          command:
            - /usr/bin/intel_fpga_admissionwebhook
          args:
            - -tls-cert-file=/etc/webhook/certs/cert.pem
            - -tls-private-key-file=/etc/webhook/certs/key.pem
            - -alsologtostderr
            - -v=2
            - 2>&1
          volumeMounts:
            - name: webhook-certs
              mountPath: /etc/webhook/certs
              readOnly: true
      volumes:
        - name: webhook-certs
          secret:
            secretName: intel-fpga-webhook-certs
---
apiVersion: v1
kind: Service
metadata:
  name: intel-fpga-webhook-svc
  labels:
    app: intel-fpga-webhook
spec:
  ports:
  - port: 443
    targetPort: 443
  selector:
    app: intel-fpga-webhook
```

create `intel-fpga-webhook-svc` service:

    $ kubectl create -f </path/to/deployment.yaml>
    deployment.extensions/intel-fpga-webhook-deployment created
    service/intel-fpga-webhook-svc created

### Configure webhook admission controller on the fly

With this spec as a template
```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: MutatingWebhookConfiguration
metadata:
  name: fpga-mutator-webhook-cfg
  labels:
    app: intel-fpga-webhook
webhooks:
- name: fpga.mutator.webhooks.intel.com
  rules:
  - apiGroups:
    - ""
    apiVersions:
    - v1
    operations:
    - CREATE
    resources:
    - pods
  clientConfig:
    service:
      path: "/pods"
      namespace: default
      name: intel-fpga-webhook-svc
    caBundle: {CA_BUNDLE}
```

register the webhook

    $ cat </path/to/fpga-pod-mutating-webhook.yaml> | sed -e "s/{CA_BUNDLE}/$(kubectl get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 -w 0)/g" | kubectl create -f -

Please note that the placeholder `{CA_BUNDLE}` should be replaced with the
certificate which is used for signing certificate requests in your cluster,
the one passed in the option `--cluster-signing-cert-file` to
`kube-controller-manager`. Depending on how your cluster is configured it may
differ from what is stored in the configmap `extension-apiserver-authentication`.
