#!/bin/sh
#
# Copyright 2019 Intel Corporation.
#
# SPDX-License-Identifier: Apache-2.0
#
# Installs a k8s cluster (single node) using cloud-native-setup scripts in
# current Jenkins test environment.

set -o pipefail
set -o xtrace
set -o errexit

sudo git clone https://github.com/clearlinux/cloud-native-setup.git
sudo bash ./cloud-native-setup/clr-k8s-examples/setup_system.sh
echo -ne '\n' |sudo bash ./cloud-native-setup/clr-k8s-examples/create_stack.sh init
sudo mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
sudo bash ./cloud-native-setup/clr-k8s-examples/create_stack.sh cni
kubectl rollout status deployment/coredns -n kube-system --timeout=5m
