#!/bin/bash -e

srcroot="$(realpath $(dirname $0)/..)"
service="intel-fpga-webhook-svc"
secret="intel-fpga-webhook-certs"

function help {
    echo "Usage: $1 <options> [help|cleanup]"
    echo '    Command "help" prints this message'
    echo '    Command "cleanup" removes admission webhook deployment'
    echo ''
    echo '    If no command is given the script will deploy the webhook'
    echo ''
    echo '    Options:'
    echo '      --kubectl <kubectl> - path to the kubectl utility'
    echo '      --mode <mode> - "preprogrammed" (default) or "orchestrated" mode of operation'
    echo '      --ca-bundle-path <path> - path to CA bundle used for signing cerificates in the cluster'
    echo '      --namespace <name> - namespace to deploy the webhook in'
}

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
	--mode)
	    mode="$2"
            shift
            ;;
        --namespace)
            namespace="$2"
            shift
            ;;
	help)
	    help $(basename $0)
	    exit 0
	    ;;
	cleanup)
	    command="cleanup"
	    ;;
	*)
	    echo "Unknown option: ${1}"
	    exit 1
	    ;;
    esac
    shift
done

[ -z ${kubectl} ] && kubectl="kubectl"
[ -z ${mode} ] && mode="preprogrammed"
[ -z ${namespace} ] && namespace="default"

# clean up any previously created deployment
${kubectl} delete MutatingWebhookConfiguration "fpga-mutator-webhook-cfg" 2>/dev/null || true
${kubectl} --namespace ${namespace} delete service ${service} 2>/dev/null || true
${kubectl} --namespace ${namespace} delete deployment "intel-fpga-webhook-deployment" 2>/dev/null || true
${kubectl} delete -f ${srcroot}/deployments/fpga_admissionwebhook/rbac-config-tpl.yaml 2>/dev/null || true
${kubectl} --namespace ${namespace} delete -f ${srcroot}/deployments/fpga_admissionwebhook/mappings-collection.yaml 2>/dev/null || true
${kubectl} --namespace ${namespace} delete -f ${srcroot}/deployments/fpga_admissionwebhook/region-crd.yaml 2>/dev/null || true
${kubectl} --namespace ${namespace} delete -f ${srcroot}/deployments/fpga_admissionwebhook/af-crd.yaml 2>/dev/null || true
${kubectl} --namespace ${namespace} delete secret ${secret} 2>/dev/null || true
${kubectl} delete csr "${service}.${namespace}" 2>/dev/null || true

if [ "x${command}" = "xcleanup" ]; then
    echo "Cleanup done. Exiting..."
    exit 0
fi

if [ "x${mode}" != "xpreprogrammed" -a "x${mode}" != "xorchestrated" ]; then
    echo "ERROR: supported modes are 'preprogrammed' and 'orchestrated'"
    exit 1
fi

if [ -z ${cabundlepath} ]; then
    CA_BUNDLE=$(${kubectl} get configmap -n kube-system extension-apiserver-authentication -o=jsonpath='{.data.client-ca-file}' | base64 -w 0)
else
    CA_BUNDLE=$(cat ${cabundlepath} | base64 -w 0)
fi

echo "Create secret including signed key/cert pair for the webhook"
${srcroot}/scripts/webhook-create-signed-cert.sh --kubectl ${kubectl} --service ${service} --secret ${secret} --namespace ${namespace}

echo "Create FPGA CRDs"
${kubectl} --namespace ${namespace} create -f ${srcroot}/deployments/fpga_admissionwebhook/af-crd.yaml
${kubectl} --namespace ${namespace} create -f ${srcroot}/deployments/fpga_admissionwebhook/region-crd.yaml
${kubectl} --namespace ${namespace} create -f ${srcroot}/deployments/fpga_admissionwebhook/mappings-collection.yaml
cat ${srcroot}/deployments/fpga_admissionwebhook/rbac-config-tpl.yaml | \
    sed -e "s/{namespace}/${namespace}/g" | \
    ${kubectl} create -f -

echo "Create webhook deployment"
cat ${srcroot}/deployments/fpga_admissionwebhook/deployment-tpl.yaml | sed -e "s/{MODE}/${mode}/g" | ${kubectl} --namespace ${namespace} create -f -

echo "Create webhook service"
${kubectl} --namespace ${namespace} create -f ${srcroot}/deployments/fpga_admissionwebhook/service.yaml

echo "Register webhook"
cat ${srcroot}/deployments/fpga_admissionwebhook/mutating-webhook-configuration-tpl.yaml | \
    sed -e "s/{CA_BUNDLE}/${CA_BUNDLE}/g" -e "s/{namespace}/${namespace}/g" | \
    ${kubectl} create -f -
