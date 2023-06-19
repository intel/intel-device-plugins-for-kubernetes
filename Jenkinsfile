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
    K8S_VERSION="1.27.1"
    GOLANGCI_LINT_VERSION="v1.52.2"
    GO_VERSION="1.20.5"
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
        label "jammy-intel-device-plugins"
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
		sh "sudo apt-get update -qq"
		sh "sudo apt-get -qq -y install libusb-1.0-0-dev buildah make gcc pkg-config"
		sh '''#!/usr/bin/env bash
		      echo -e 'unqualified-search-registries = ["docker.io"]' | sudo tee -a /etc/containers/registries.conf
		'''
		sh "sudo curl -L https://dl.k8s.io/release/v${K8S_VERSION}/bin/linux/amd64/kubectl -o /usr/bin/kubectl"
		sh "sudo chmod +x /usr/bin/kubectl"   
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
            stage("make terrascan") {
              steps {
                dir(path: "$REPO_DIR") {
                  sh "curl -sL `curl -sL https://api.github.com/repos/accurics/terrascan/releases/latest | grep -o -E https://.+?_Linux_x86_64.tar.gz` | tar -zx terrascan"
                  sh "sudo mv terrascan /usr/local/bin/"
                  sh "make terrascan"
                }
              }
            }
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
        stage('make test-with-kind') {
          steps {
            dir(path: "$REPO_DIR") {
              sh "make test-with-kind REG=intel/ TAG=0.27.0"
            }
          }
        }
        stage('push images') {
          when { not { changeRequest() } }
          steps {
            withDockerRegistry([ credentialsId: "e16bd38a-76cb-4900-a5cb-7f6aa3aeb22d", url: "https://${REG}" ]) {
              sh "make push"
            }
          }
        }
      }
    }
  }
}
