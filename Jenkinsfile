def skus = ["qat-c62x", "qat-c3xxx"]

def parallelStagesMap = skus.collectEntries {
  ["${it}" : generateStage(it)]
}

def generateStage(job) {
  return {
    node("${job}") {
      stage('Tests') {
        checkout scm
        sh 'make set-version'
        try {
          sh 'make -C ./scripts/jenkins tests'
        }
        catch(e) {
          throw e
        }
        finally {
          sh 'make -C ./scripts/jenkins logs'
        }
      }
    }
  }
}

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
    GO_VERSION="1.13.5"
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
    stage('Intel Device plugins') {
      when { changeRequest() }
      steps {
        script {
          parallel parallelStagesMap
        }
      }
    }
  }
}
