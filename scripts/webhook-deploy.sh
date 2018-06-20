#!/bin/bash -e

# Check we've got all dependencies installed
which cfssl > /dev/null
which jq > /dev/null

srcroot="$(realpath $(dirname $0)/..)"

while [[ $# -gt 0 ]]; do
    case ${1} in
	--kubectl)
	    kubectl="$2"
            shift
            ;;
	--ca-bundle-path)
	    cabundlepath="$2"
            shift
            ;;
    esac
    shift
done

[ -z ${kubectl} ] && kubectl="kubectl"

if [ -z ${cabundlepath} ]; then
    CA_BUNDLE=$(${kubectl} get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 -w 0)
else
    CA_BUNDLE=$(cat ${cabundlepath} | base64 -w 0)
fi

echo "Create secret including signed key/cert pair for the webhook"
${srcroot}/scripts/webhook-create-signed-cert.sh --kubectl ${kubectl}

echo "Create webhook deployment"
kubectl create -f ${srcroot}/deployments/fpga_admissionwebhook/deployment.yaml

echo "Create webhook service"
kubectl create -f ${srcroot}/deployments/fpga_admissionwebhook/service.yaml

echo "Register webhook"
cat ${srcroot}/deployments/fpga_admissionwebhook/mutating-webhook-configuration-tpl.yaml | sed -e "s/{CA_BUNDLE}/${CA_BUNDLE}/g" | ${kubectl} create -f -
