pipeline {
  agent {
    label "master"
  }
  options {
    timeout(time: 3, unit: "HOURS")
  }
  environment {
    GO111MODULE="on"
    REG="cloud-native-image-registry.westus.cloudapp.azure.com/"
    RUNC_VERSION="v1.0.0-rc8"
    CRIO_VERSION="v1.14.6"
    BUILDAH_VERSION="v1.10.0"
    GO_VERSION="1.12.9"
    GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
    GOROOT="/usr/local/go"
    GOPATH="/tmp/go"
    PATH="${env.PATH}:/usr/local/go/bin:${GOPATH}/bin"
    REPO_NAME="intel-device-plugins-for-kubernetes"
    REPO_DIR="$GOPATH/src/github.com/intel/${REPO_NAME}"
  }
  stages {
    stage("Set env") {
      steps {
        script {
          if (env.CHANGE_ID == null) {
            env.PR = 'no'
          }
          else {
            env.TAG = env.BUILD_TAG + '-rejected'
            env.PR = 'yes'
          }
        }
      }
    }
    stage("Build && Publish") {
      agent {
        label "xenial-intel-device-plugins"
      }
      stages {
        stage("Get requirements") {
          parallel {
            stage("go") {
              steps {
                sh "curl -O https://dl.google.com/go/${GO_TAR}"
                sh "tar -xvf $GO_TAR"
                sh "sudo mv go $GOROOT"
                sh "mkdir -p $GOPATH/src/github.com/intel"
                sh "cp -rf ${env.WORKSPACE} $REPO_DIR"
                dir(path: "$REPO_DIR") {
                  sh "go get -v golang.org/x/lint/golint"
                  sh "go get -v golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow"
                  sh "go get -v github.com/fzipp/gocyclo"
                }
              }
            }
            stage("buildah") {
              steps {
                sh "sudo apt-get update"
                sh "sudo apt-get -y install e2fslibs-dev libfuse-dev libgpgme11-dev libdevmapper-dev libglib2.0-dev libprotobuf-dev"
                sh "mkdir -p ${GOPATH}/src/github.com/containers"
                dir(path: "${GOPATH}/src/github.com/containers") {
                  sh "git clone --single-branch --depth 1 -b $BUILDAH_VERSION https://github.com/containers/buildah"
                }
                dir(path: "${GOPATH}/src/github.com/containers/buildah") {
                  sh 'make buildah TAGS=""'
                  sh "sudo cp buildah /usr/local/bin"
                  sh "sudo mkdir -p /etc/containers"
                  sh '''echo '[registries.search]' > registries.conf'''
                  sh '''echo 'registries = ["docker.io"]' >> registries.conf'''
                  sh "sudo mv registries.conf /etc/containers/registries.conf"
                  sh "sudo curl https://raw.githubusercontent.com/kubernetes-sigs/cri-o/$CRIO_VERSION/test/policy.json -o /etc/containers/policy.json"
                  sh "sudo curl -L https://github.com/opencontainers/runc/releases/download/$RUNC_VERSION/runc.amd64 -o /usr/bin/runc"
                  sh "sudo chmod +x /usr/bin/runc"
                }
              }
            }
          }
        }
        stage("make vet, lint, cyclomatic"){
          parallel {
            stage("make lint") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make lint"
                }
              }
            }
            stage("make format") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make format"
                }
              }
            }
            stage("make vet") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make vet"
                }
              }
            }
            stage("make cyclomatic-check") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make cyclomatic-check"
                }
              }
            }
            stage("make test BUILDTAGS=kerneldrv") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make test BUILDTAGS=kerneldrv"
                }
              }
            }
          }
        }
        stage('make pre-pull') {
          steps {
    	      dir(path: "$REPO_DIR") {
              sh "make pre-pull"
            }
          }
        }
        stage('make images') {
          parallel {
            stage("make images with docker") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make -j4 images"
                }
              }
            }
            stage("make images with buildah") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make images BUILDER=buildah"
                }
              }
            }
          }
        }
        stage('make demos') {
          parallel {
            stage('make demos with docker') {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make demos"
                }
              }
            }
            stage('make demos with buildah') {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make demos BUILDER=buildah"
                }
              }
            }
          }
        }
      }
      post {
        success {
          withDockerRegistry([ credentialsId: "e16bd38a-76cb-4900-a5cb-7f6aa3aeb22d", url: "https://${REG}" ]) {
            sh "make push"
          }
        }
      }
    }
    stage('Intel QAT plugin') {
      when {
        environment name: 'PR', value: 'yes'
      }
      agent {
        label "qat-clearlinux"
      }
      stages {
        stage('Checks') {
          steps {
            sh 'sudo lsmod'
            sh 'sudo dmesg | grep -i qat'
            sh 'sudo swupd bundle-list | grep -i qat'
            sh 'sudo swupd bundle-list | grep -i dpdk'
            sh 'sudo cat /proc/cmdline'
            sh 'for i in 0434 0435 37c8 6f54 19e2; do sudo lspci -D -d 8086:$i; done'
            sh 'for i in 0442 0443 37c9 19e3; do sudo lspci -D -d 8086:$i; done'
          }
        }
        stage('Install K8s') {
          steps {
            sh 'sudo git clone https://github.com/clearlinux/cloud-native-setup.git'
            sh 'sudo bash ./cloud-native-setup/clr-k8s-examples/setup_system.sh'
            sh 'sudo bash ./cloud-native-setup/clr-k8s-examples/create_stack.sh init'
            sh 'sudo mkdir -p $HOME/.kube'
            sh 'sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config'
            sh 'sudo chown $(id -u):$(id -g) $HOME/.kube/config'
            sh 'sudo bash ./cloud-native-setup/clr-k8s-examples/create_stack.sh cni'
            sh 'kubectl rollout status deployment/coredns -n kube-system --timeout=5m'
          }
        }
        stage('Pull images') {
          steps {
            withCredentials([usernamePassword(credentialsId: 'e16bd38a-76cb-4900-a5cb-7f6aa3aeb22d', passwordVariable: 'RPASS', usernameVariable: 'RUSER')]) {
              sh 'sudo crictl pull --creds ${RUSER}:${RPASS} ${REG}intel-qat-plugin:${TAG}'
              sh 'sudo crictl pull --creds ${RUSER}:${RPASS} ${REG}crypto-perf:${TAG}'
            }
          }
        }
        stage('Deploy QAT plugin') {
          steps {
            sh 'sed -i "s#intel/intel-qat-plugin:devel#${REG}intel-qat-plugin:${TAG}#g" ./deployments/qat_plugin/qat_plugin.yaml'
            sh 'sed -i "s#intel/crypto-perf:devel#${REG}crypto-perf:${TAG}#g" ./deployments/qat_dpdk_app/base/crypto-perf-dpdk-pod-requesting-qat.yaml'
            sh 'kubectl create -f ./deployments/qat_plugin/qat_plugin_default_configmap.yaml'
            sh 'kubectl create -f ./deployments/qat_plugin/qat_plugin.yaml'
            sh 'kubectl rollout status ds/intel-qat-plugin --timeout=5m'
            sh 'kubectl wait --for=condition=Ready pod --all --timeout=5m && sleep 60s'
          }
        }
        stage('DPDK app tests') {
          parallel {
            stage('Run crypto-tc1') {
              steps {
                sh 'kubectl apply -k ./deployments/qat_dpdk_app/test-crypto1/'
                sh 'kubectl wait --for=condition=Initialized pod/qat-dpdk-test-crypto-perf-tc1 --timeout=5m && sleep 60s'
                sh 'kubectl logs -f qat-dpdk-test-crypto-perf-tc1 | tee qat-dpdk-test-crypto-perf-tc1.log'
              }
            }
            stage('Run compress-tc1') {
              steps {
                sh 'kubectl apply -k ./deployments/qat_dpdk_app/test-compress1/'
                sh 'kubectl wait --for=condition=Initialized pod/qat-dpdk-test-compress-perf-tc1 --timeout=5m && sleep 60s'
                sh 'kubectl logs -f qat-dpdk-test-compress-perf-tc1 | tee qat-dpdk-test-compress-perf-tc1.log'
              }
            }
          }
        }
      }
    }
  }
}
