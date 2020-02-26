#!/bin/bash

# This script prepares fpga_plugin ready for full deployment by
#
# kubectl apply -k deployments/fpga_plugin/overlays/af
#
# or
#
# kubectl apply -k deployments/fpga_plugin/overlays/region

service="intel-fpga-webhook-svc"
secret="intel-fpga-webhook-certs"

script_dir="$(realpath $(dirname $0))"
srcroot="$(realpath ${script_dir}/..)"

kustomize_secret_dir="${srcroot}/deployments/fpga_admissionwebhook/base/${secret}-secret"

mkdir -p "${kustomize_secret_dir}"

# Create signed cert files to kustomize_secret_dir
${script_dir}/webhook-create-signed-cert.sh --output-dir ${kustomize_secret_dir} --service $service && {
    echo ""
    echo created for kustomization:
    echo - "${kustomize_secret_dir}"
}
