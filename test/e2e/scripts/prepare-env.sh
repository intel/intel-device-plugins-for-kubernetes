#!/bin/bash

script_path=$(dirname "$(readlink -f "$0")")

source "$script_path/common.sh"

cd "$GITHUB_WORKSPACE" || exit 1

fetch_current_k8s_version || exit 1

echo "$K8S_VERSION"

k3s_version_for_k8s_version || exit 1

echo "$K3S_VERSION"

download_k3s_binaries || exit 1

sudo cp $collateral_path/k3s-cache/$K3S_VERSION/k3s /usr/local/bin/k3s || exit 1

prepare_cluster && install_k8s_deps && cache_shared_images || exit 1

exit 0
