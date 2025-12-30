#!/bin/bash

collateral_path=$HOME/collaterals/

K8S_VERSION=""
K3S_VERSION=""

fetch_current_k8s_version() {
  local version

  version=$(yq .jobs.envtest.strategy.matrix.version[-1] $GITHUB_WORKSPACE/.github/workflows/lib-validate.yaml)

  # cut the ".x" from the version
  version=$(echo $version | tr -d '"' | sed 's/\.x$//')

  if [ -z "$version" ]; then
    echo "Couldn't find k8s version in the workflow file"

    return 1
  fi

  K8S_VERSION=$version
}

k3s_version_for_k8s_version() {
  local requested="$K8S_VERSION"

  local known_versions

  known_versions="v1.35.0+k3s1;v1.34.1+k3s1;v1.33.5+k3s1;v1.32.9+k3s1;v1.31.9+k3s1;v1.30.13+k3s1"

  local latest
  latest=$(echo $known_versions | tr ';' '\n' | grep "$requested" | head -1)
  if [ -z "$latest" ]; then
    echo "No k3s version found for requested k8s version $requested"

    return 1
  fi

  K3S_VERSION=$latest
}

download_k3s_binaries() {
  mkdir -p $collateral_path/k3s-cache

  [ -e $collateral_path/k3s-cache/install-k3s.sh ] || {
    wget https://get.k3s.io/ -O $collateral_path/k3s-cache/install-k3s.sh || {
      echo "Failed to download k3s install script"

      return 1
    }

    chmod +x $collateral_path/k3s-cache/install-k3s.sh
  }

  [ -e $collateral_path/k3s-cache/${K3S_VERSION} ] && {
    echo "Using cached k3s binary"

    return 0
  }

  local k3s_ver_encoded

  k3s_ver_encoded=$(echo "$K3S_VERSION" | sed -e 's/+/\%2B/')

  local k3s_url
  k3s_url="https://github.com/k3s-io/k3s/releases/download/${k3s_ver_encoded}/k3s"

  local k3s_images_url
  k3s_images_url="https://github.com/k3s-io/k3s/releases/download/${k3s_ver_encoded}/k3s-airgap-images-amd64.tar.zst"

  mkdir -p $collateral_path/k3s-cache/${K3S_VERSION}

  wget -q -O $collateral_path/k3s-cache/${K3S_VERSION}/k3s $k3s_url || {
    echo "Failed to download k3s binary from $k3s_url"
    return 1
  }

  chmod +x $collateral_path/k3s-cache/${K3S_VERSION}/k3s

  wget -q -O $collateral_path/k3s-cache/${K3S_VERSION}/images.tar.zst $k3s_images_url || {
    echo "Failed to download k3s images from $k3s_images_url"
    return 1
  }

  return 0
}

print_large() {
  type figlet > /dev/null 2>&1 && {
    figlet "$@"
  } || {
    echo "========================================"
    echo "$@"
    echo "========================================"
  }
}

wait_for_cluster_to_be_ready() {
  echo "Waiting for cluster to become accessible"

  for _ in $(seq 60); do
    kubectl get pods -A 2>&1 | grep -q "No resources found" || {
      echo "Cluster is accessible"
      break
    }

    echo -n "."
    sleep 1
  done

  echo "Waiting for Pods to become ready"

  for _ in $(seq 60); do
    sleep 1
    echo -n "."

    allcount=$(kubectl get pods -A --no-headers=true | wc -l)
    notreadycount=$(kubectl get pods -A --no-headers=true | grep -c -v -e Complete -e Running)

    if [ $allcount -lt 7 ]; then
      continue
    fi

    if [ $notreadycount -eq 0 ]; then
      echo "READY"

      return 0
    fi
  done

  echo ""
  echo "Cluster did not become ready.."

  return 1
}

prepare_cluster() {
  print_large "Prepare cluster"

  echo "Versions: $K3S_VERSION & $K8S_VERSION"

  [ -e /usr/local/bin/k3s-uninstall.sh ] && {
    echo "Found existing k3s install, removing it"

    k3s-uninstall.sh || return 1
  }

  echo "prepare images"
  sudo mkdir -p /var/lib/rancher/k3s/agent/images/ && \
  sudo cp $collateral_path/k3s-cache/$K3S_VERSION/images.tar.zst /var/lib/rancher/k3s/agent/images/k3s-airgap-images-amd64.tar.zst || return 1
  sudo cp $collateral_path/k3s-cache/$K3S_VERSION/k3s /usr/local/bin/ || return 1
  sudo chmod +x /usr/local/bin/k3s || return 1

  echo "prepare kubelet config"
  cat <<EOF > /tmp/kubelet.conf
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
cpuManagerPolicy: static
systemReserved:
  cpu: 2000m
  memory: 512M
kubeReserved:
  cpu: 1000m
  memory: 256M
EOF

  sudo mkdir -p /etc/rancher/k3s
  cat <<EOF > /tmp/k3s-config.yaml
kubelet-arg:
  - config=/tmp/kubelet.conf
EOF
  sudo mv /tmp/k3s-config.yaml /etc/rancher/k3s/config.yaml

  echo "prepare k3s cluster"
  INSTALL_K3S_SKIP_DOWNLOAD=true INSTALL_K3S_EXEC='--write-kubeconfig-mode=644' $collateral_path/k3s-cache/install-k3s.sh && \
  sudo chmod +r /etc/rancher/k3s/k3s.yaml && \
  sudo chmod o+rw /run/k3s/containerd/containerd.sock || {
    return 1
  }

  export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

  wait_for_cluster_to_be_ready || return 1

  kubectl get nodes -o wide
  kubectl get pods -A
}

install_go() {
  local go_version
  go_version=$(grep "^go " $GITHUB_WORKSPACE/go.mod | head -1 | cut -c 4-)

  [ -e $collateral_path/go$go_version.linux-amd64.tar.gz ] || {
    wget https://go.dev/dl/go$go_version.linux-amd64.tar.gz -O $collateral_path/go$go_version.linux-amd64.tar.gz || exit 1
  }

  mkdir -p ~/bin
  tar -xf $collateral_path/go$go_version.linux-amd64.tar.gz -C ~/bin || exit 1

  export PATH=$PATH:~/bin/go/bin

  type go || {
    echo "Go installation failed"
    return 1
  }

  go version || return 1
}

install_k8s_deps() {
  print_large "Install cert-manager"
  kubectl apply --wait -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.0/cert-manager.yaml || return 1
}

prepare_to_build() {
  echo "Fetch vendor dependencies"
  make vendor || return 1

  echo "Generate dummy licenses"

  for ldir in $(ls --ignore=internal cmd | xargs -I"{}" echo licenses/{}); do
    mkdir -p $GITHUB_WORKSPACE/$ldir
    touch $GITHUB_WORKSPACE/$ldir/dummy-license.txt
  done
}

# This should be executed only once per run.
generate_tag() {
  # Extract the version from the reconciler.
  local BUILD_VERSION
  BUILD_VERSION=$(grep -r --include="*.go" 'ImageMinVersion =' ${GITHUB_WORKSPACE} | head -1 | sed -e 's/.*"\(.*\)".*/\1/')

  # Add random components to avoid collision with other images.
  BUILD_VERSION=$BUILD_VERSION-$GITHUB_RUN_NUMBER-$RANDOM

  export TAG=$BUILD_VERSION
}

cache_shared_images() {
  echo "Cache shared images"

  local SHARED_IMAGES=""

  echo $IMAGES | grep -q gpu && {
    SHARED_IMAGES="${SHARED_IMAGES}intel/intel-extension-for-pytorch:2.8.10-xpu "
  }

  for image in $SHARED_IMAGES;
  do
    echo "Downloading and caching $image"
    docker pull $image && docker save $image | sudo ctr -n k8s.io image import - || return 1
  done
}
