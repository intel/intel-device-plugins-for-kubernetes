pipeline {
  agent {
    label "xenial-intel-device-plugins"
  }
  options {
    timeout(time: 2, unit: "HOURS")
  }
  environment {
    CUSTOM_REGISTRY = "cloud-native-image-registry.westus.cloudapp.azure.com"
    CUSTOM_TAG = "${env.BUILD_TAG}-rejected"
    RUNC_VERSION="v1.0.0-rc8"
    CRIO_VERSION="v1.14.6"
    BUILDAH_VERSION="v1.10.0"
    GO_VERSION="1.12.8"
    GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
    GOROOT="/usr/local/go"
    GOPATH="/tmp/go"
    PATH="${env.PATH}:/usr/local/go/bin:${GOPATH}/bin"
    REPO_NAME="intel-device-plugins-for-kubernetes"
    REPO_DIR="$GOPATH/src/github.com/intel/${REPO_NAME}"
  }
  stages {
    stage('Builds') {
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
        stage("make vet, lint, cyclomatic") {
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
        stage('make images') {
          parallel {
            stage("make images with docker") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make images"
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
        stage('make push images') {
          steps {
            withDockerRegistry([credentialsId: "57e4a8b2-ccf9-4da1-a787-76dd1aac8fd1", url: "https://${CUSTOM_REGISTRY}"]) {
              dir(path: "$REPO_DIR") {
                sh "make push-all"
              }
            }
          }
        }
      }
    }
    stage('Intel QAT plugin') {
      agent {
        label "qat-dpdk"
      }
      stages {
        stage('Checks') {
          steps {
            sh 'cat /sys/kernel/mm/transparent_hugepage/enabled'
            sh 'cat /bootmp/loader/entries/Clear-linux-lts2018-4.19.63-67.conf'
            sh 'lsmod | grep -i qat'
            sh 'lsmod | grep -i vfio'
            sh 'lspci -d:37c8'
            sh 'lspci -d:37c9'
            sh 'kubectl delete -k ./deployments/qat_dpdk_app/test-crypto1/ --ignore-not-found=true'
            sh 'kubectl delete -f ./deployments/qat_plugin/ --ignore-not-found=true'
            sh 'docker system prune -a -f'
          }
        }
        stage('Pull && Load images') {
          steps {
            withDockerRegistry([credentialsId: "57e4a8b2-ccf9-4da1-a787-76dd1aac8fd1", url: "https://${CUSTOM_REGISTRY}"]) {
              sh 'docker pull $CUSTOM_REGISTRY/intel-qat-plugin:$CUSTOM_TAG'
              sh 'docker pull $CUSTOM_REGISTRY/crypto-perf:$CUSTOM_TAG'
            }
            sh 'docker tag $CUSTOM_REGISTRY/intel-qat-plugin:$CUSTOM_TAG intel-qat-plugin:devel'
            sh 'docker tag $CUSTOM_REGISTRY/crypto-perf:$CUSTOM_TAG crypto-perf:devel'
          }
        }
        stage('Deploy QAT plugin') {
         steps {
           sh 'sed -i "s#/usr/bin/intel_qat_device_plugin#/usr/local/bin/intel_qat_device_plugin#g" ./deployments/qat_plugin/qat_plugin.yaml'
           sh 'kubectl create -f ./deployments/qat_plugin/qat_plugin_default_configmap.yaml'
           sh 'kubectl create -f ./deployments/qat_plugin/qat_plugin.yaml'
           sh 'kubectl rollout status ds/intel-qat-plugin --timeout=5m'
           sh 'kubectl wait --for=condition=Ready pod --all --timeout=5m'
         }
        }
        stage('Run crypto-perf-tc1') {
          steps {
            sh 'kubectl apply -k ./deployments/qat_dpdk_app/test-crypto1/'
            sh 'kubectl wait --for=condition=Initialized pod/qat-dpdk-test-crypto-perf-tc1 --timeout=5m && sleep 10s'
            sh 'kubectl logs -f qat-dpdk-test-crypto-perf-tc1 | tee qat-dpdk-test-crypto-perf-tc1.log'
          }
        }
      }
      post {
        always {
          deleteDir()
          sh 'kubectl delete -k ./deployments/qat_dpdk_app/test-crypto1/ --ignore-not-found=true'
          sh 'kubectl delete -f ./deployments/qat_plugin/ --ignore-not-found=true'
        }
        success {
          sh 'docker tag intel-qat-plugin:devel $CUSTOM_REGISTRY/intel-qat-plugin:devel'
          sh 'docker tag crypto-perf:devel $CUSTOM_REGISTRY/crypto-perf:devel'
          withDockerRegistry([credentialsId: "57e4a8b2-ccf9-4da1-a787-76dd1aac8fd1", url: "https://${CUSTOM_REGISTRY}"]) {
            sh 'CUSTOM_TAG="devel" make push-intel-qat-plugin'
            sh 'CUSTOM_TAG="devel" make push-crypto-perf'
          }
        }
      }
    }
  }
}
