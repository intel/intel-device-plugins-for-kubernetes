#!/bin/bash

which cfssl > /dev/null 2>&1 || { echo "Please install 'cfssl' (e.g. with 'go get -u github.com/cloudflare/cfssl/cmd/cfssl')"; exit 1; }
which jq > /dev/null 2>&1 || { echo "Please install 'jq'"; exit 1; }

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
        --output-dir)
            output_dir="$2"
            shift
            ;;
    esac
    shift
done

[ -z ${service} ] && service="webhook-svc"
[ -z ${secret} ] && secret="webhook-certs"
[ -z ${namespace} ] && namespace="default"
[ -z ${kubectl} ] && kubectl="kubectl"
[ -z ${output_dir} ] && output_dir=""

which ${kubectl} > /dev/null 2>&1 || { echo "ERROR: ${kubectl} not found"; exit 1; }

csrname="${service}.${namespace}"
tmpdir=$(mktemp -d)

echo "Creating certs in ${tmpdir}"

cat <<EOF >> ${tmpdir}/csr-config.json
{
    "CN": "${service}.${namespace}.svc",
    "hosts": [
        "${service}",
        "${service}.${namespace}",
        "${service}.${namespace}.svc"
    ],
    "key": {
        "algo": "rsa",
        "size": 2048
    }
}
EOF

cfssl genkey -loglevel=2 ${tmpdir}/csr-config.json > ${tmpdir}/csr.json
jq --raw-output '.key' ${tmpdir}/csr.json > ${tmpdir}/server-key.pem
jq --raw-output '.csr' ${tmpdir}/csr.json > ${tmpdir}/server.csr

# clean-up any previously created CSR for our service. Ignore errors if not present.
${kubectl} delete csr ${csrname} 2>/dev/null || true

# create  server cert/key CSR and  send to k8s API
cat <<EOF | ${kubectl} create -f -
apiVersion: certificates.k8s.io/v1beta1
kind: CertificateSigningRequest
metadata:
  name: ${csrname}
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
    ${kubectl} get csr ${csrname}
    if [ "$?" -eq 0 ]; then
        break
    fi
done

# approve and fetch the signed certificate
${kubectl} certificate approve ${csrname}

# verify certificate has been signed
for x in $(seq 10); do
    serverCert=$(${kubectl} get csr ${csrname} -o jsonpath='{.status.certificate}')
    if [[ ${serverCert} != '' ]]; then
        break
    fi
    sleep 1
done

if [[ ${serverCert} == '' ]]; then
    echo "ERROR: After approving csr ${csrname}, the signed certificate did not appear on the resource. Giving up after 10 attempts." >&2
    exit 1
fi

echo ${serverCert} | base64 --decode > ${tmpdir}/server-cert.pem

# clean-up any previously created secret for our service. Ignore errors if not present.
${kubectl} delete secret ${secret} 2>/dev/null || true

if [ -z "${output_dir}" ]; then
    # create the secret with CA cert and server cert/key
    ${kubectl} create secret generic ${secret} \
               --from-file=key.pem=${tmpdir}/server-key.pem \
               --from-file=cert.pem=${tmpdir}/server-cert.pem \
               --dry-run -o yaml |
        ${kubectl} -n ${namespace} apply -f -
else
    # save CA cert and server cert/key to output_dir
    ( cp ${tmpdir}/server-key.pem ${output_dir}/key.pem &&
      cp ${tmpdir}/server-cert.pem ${output_dir}/cert.pem ) || {
        echo "ERROR: failed to copy ${tmpdir}/server-{key,cert}.pem to output_dir \"${output_dir}\""
        exit 1
    }
    ${kubectl} get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' > "${output_dir}/client-ca-file" || {
        echo "ERROR: failed to save extension-apiserver-authentication.client-ca-file to output_dir \"${output_dir}\""
        exit 1
    }
fi

echo "Removing ${tmpdir}"
rm -rf ${tmpdir}
