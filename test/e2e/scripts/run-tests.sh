#!/bin/bash

script_path=$(dirname "$(readlink -f "$0")")
source "$script_path/common.sh"

export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
export PATH=$PATH:~/bin/go/bin

echo "Running tests: $TARGET_JOB"

make "$TARGET_JOB" || {
  exit 1
}

exit 0
