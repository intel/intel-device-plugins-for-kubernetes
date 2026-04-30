#!/bin/bash

script_path=$(dirname "$(readlink -f "$0")")
source "$script_path/common.sh"

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
export PATH=$PATH:~/bin/go/bin

export PLUGINS_REPO_DIR=${GITHUB_WORKSPACE}

export PROJECT_NAMESPACE=inteldeviceplugins-system
export IMAGE_PATH=intel
export PLUGIN_VERSION=${TAG}
export IMAGE_REGISTRY=docker.io

echo "Running tests: $TARGET_JOB"

make "$TARGET_JOB" || {
  exit 1
}

exit 0
