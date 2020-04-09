#!/bin/sh -eu

srcroot="$(realpath $(dirname $0)/..)"
kubectl='kubectl'
namespace='default'
mode='af'
command=''

help() {
    echo "Usage: $1 <options> [help|cleanup]"
    echo '    Command "help" prints this message'
    echo '    Command "cleanup" removes admission webhook deployment'
    echo ''
    echo '    If no command is given the script will deploy the webhook'
    echo ''
    echo '    Options:'
    echo '      --kubectl <kubectl> - path to the kubectl utility'
    echo '      --mode <mode> - "af" (default) or "region" mode of operation'
    echo '      --namespace <name> - namespace to deploy the plugin in'
}

while [ $# -gt 0 ]; do
    case ${1} in
        --kubectl)
            kubectl="$2"
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

which ${kubectl} > /dev/null 2>&1 || { echo "ERROR: ${kubectl} not found"; exit 1; }

echo "Clean up previously created deployment"
${kubectl} annotate node --all fpga.intel.com/device-plugin-mode- || true
sed -e "s/{namespace}/${namespace}/g" ${srcroot}/deployments/fpga_plugin/fpga_plugin.yaml | ${kubectl} --namespace ${namespace} delete -f - || true
sed -e "s/{namespace}/${namespace}/g" ${srcroot}/deployments/fpga_plugin/fpga_plugin_service_account.yaml | ${kubectl} --namespace ${namespace} delete -f - || true

if [ "x${command}" = "xcleanup" ]; then
    echo "Cleanup done. Exiting..."
    exit 0
fi

if [ "x${mode}" != "xaf" -a "x${mode}" != "xregion" ]; then
    echo "ERROR: supported modes are 'af' and 'region'"
    exit 1
fi

echo 'Set default operation mode'
${kubectl} annotate node --overwrite --all fpga.intel.com/device-plugin-mode=${mode} || true
echo 'Create service account'
sed -e "s/{namespace}/${namespace}/g" ${srcroot}/deployments/fpga_plugin/fpga_plugin_service_account.yaml | ${kubectl} --namespace ${namespace} create -f - || true
echo 'Create plugin daemonset'
sed -e "s/{namespace}/${namespace}/g" ${srcroot}/deployments/fpga_plugin/fpga_plugin.yaml | ${kubectl} --namespace ${namespace} create -f - || true
