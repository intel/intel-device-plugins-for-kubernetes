pipeline {
  agent {
    label "master"
  }
  options {
    timeout(time: 4, unit: "HOURS")
  }
  environment {
    GO111MODULE="on"
    REG="cloud-native-image-registry.westus.cloudapp.azure.com/"
    RUNC_VERSION="v1.0.3"
    CRIO_VERSION="v1.21.4"
    K8S_VERSION="1.22.1"
    GOLANGCI_LINT_VERSION="v1.43.0"
    GO_VERSION="1.17.4"
    GO_TAR="go${GO_VERSION}.linux-amd64.tar.gz"
    GOROOT="/usr/local/go"
    GOPATH="/tmp/go"
    PATH="${env.PATH}:/usr/local/go/bin:${GOPATH}/bin"
    REPO_NAME="intel-device-plugins-for-kubernetes"
    REPO_DIR="$GOPATH/src/github.com/intel/${REPO_NAME}"
  }
  stages {
    stage("Set env") {
      when { changeRequest() }
      steps {
        script {
          env.TAG = env.CHANGE_ID + '-rejected'
        }
      }
    }
    stage("Build && Publish") {
      agent {
        label "bionic-intel-device-plugins"
      }
      stages {
        stage("Get requirements") {
              steps {
                sh "curl -O https://dl.google.com/go/${GO_TAR}"
                sh "tar -xvf $GO_TAR"
                sh "sudo mv go $GOROOT"
                sh "mkdir -p $GOPATH/src/github.com/intel $GOPATH/bin"
                sh "cp -rf ${env.WORKSPACE} $REPO_DIR"
                sh "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH}/bin ${GOLANGCI_LINT_VERSION}"
		sh '''#!/usr/bin/env bash
		   . /etc/os-release
		   REPOURL=http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/x${ID^}_${VERSION_ID}
		   echo "deb ${REPOURL} /" | sudo tee /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list
		   wget -nv ${REPOURL}/Release.key -O - | sudo apt-key add -
                '''
		sh "sudo apt-get update -qq"
		sh "sudo apt-get -qq -y install libusb-1.0-0-dev buildah make gcc pkg-config"
		sh "sudo curl https://raw.githubusercontent.com/cri-o/cri-o/${CRIO_VERSION}/test/registries.conf -o /etc/containers/registries.conf"
		sh "sudo sed -i -e 's/quay/docker/' /etc/containers/registries.conf"
		sh "sudo curl -L https://github.com/opencontainers/runc/releases/download/$RUNC_VERSION/runc.amd64 -o /usr/bin/runc"
		sh "sudo chmod +x /usr/bin/runc"
              }
        }
        stage("make go-mod-tidy") {
          steps {
            dir(path: "$REPO_DIR") {
              sh "make go-mod-tidy"
            }
          }
        }
        stage("make lint"){
          parallel {
            stage("make lint") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "make lint"
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
            stage('make envtest') {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest"
                  sh "setup-envtest use ${K8S_VERSION}"
                  sh '''#!/usr/bin/env bash
                     source <(setup-envtest use -p env ${K8S_VERSION})
                     make envtest
                  '''
                }
              }
            }
          }
        }
        stage('make pre-pull') {
          steps {
            dir(path: "$REPO_DIR") {
              sh "make pre-pull"
              sh "make -e vendor"
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
  }
}
