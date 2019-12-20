#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Deploys current jenkins build test images 'intel-fpga-plugin' and
# 'intel-fpga-webhook-deployment' into the cluster namespace 'kube-system'

set -o pipefail
set -o xtrace
set -o errexit

REPO_ROOT=$(realpath $(dirname $0)/../../..)
go get -u github.com/cloudflare/cfssl/cmd/cfssl
cd ${REPO_ROOT}/scripts ; ./webhook-deploy.sh --namespace kube-system --mode orchestrated # Deploy webhook
kubectl create -f ${REPO_ROOT}/deployments/fpga_plugin/fpga_plugin_service_account.yaml # Create service account for the plugin 
kubectl annotate node --all 'fpga.intel.com/device-plugin-mode=region' # Set region mode for the plugin 
kubectl create -f ${REPO_ROOT}/deployments/fpga_plugin/fpga_plugin.yaml # Create plugin daemonset 
kubectl create -f ${REPO_ROOT}/deployments/fpga_admissionwebhook/mappings-collection.yaml 
kubectl wait --for=condition=Ready pod --all --timeout=5m --namespace kube-system && sleep 3m

# Plugin deployment verification
kubectl get pods --namespace kube-system | grep intel-fpga-webhook # Check if pod is running
kubectl get pod --namespace kube-system | grep intel-fpga-plugin # Check if pod is running
kubectl logs --namespace kube-system $(kubectl get pod --namespace kube-system | grep intel-fpga-plugin | awk '{print $1}') # Check if pod is running in region mode 
kubectl describe node | grep -A5 Allocatable # Check if resource fpga.intel.com/region-<FPGA interface id> is allocatable