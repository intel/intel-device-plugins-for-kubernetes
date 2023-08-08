## This is a generated file, do not edit directly. Edit build/docker/templates/intel-sgx-initcontainer.Dockerfile.in instead.
##
## Copyright 2022 Intel Corporation. All Rights Reserved.
##
## Licensed under the Apache License, Version 2.0 (the "License");
## you may not use this file except in compliance with the License.
## You may obtain a copy of the License at
##
## http://www.apache.org/licenses/LICENSE-2.0
##
## Unless required by applicable law or agreed to in writing, software
## distributed under the License is distributed on an "AS IS" BASIS,
## WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
## See the License for the specific language governing permissions and
## limitations under the License.
###
## FINAL_BASE can be used to configure the base image of the final image.
##
## This is used in two ways:
## 1) make <image-name> BUILDER=<docker|buildah>
## 2) docker build ... -f <image-name>.Dockerfile
##
## The project default is 1) which sets FINAL_BASE=gcr.io/distroless/static
## (see build-image.sh).
## 2) and the default FINAL_BASE is primarily used to build Redhat Certified Openshift Operator container images that must be UBI based.
## The RedHat build tool does not allow additional image build parameters.
ARG FINAL_BASE=registry.access.redhat.com/ubi8-micro:latest
###
##
## GOLANG_BASE can be used to make the build reproducible by choosing an
## image by its hash:
## GOLANG_BASE=golang@sha256:9d64369fd3c633df71d7465d67d43f63bb31192193e671742fa1c26ebc3a6210
##
## This is used on release branches before tagging a stable version.
## The main branch defaults to using the latest Golang base image.
ARG GOLANG_BASE=golang:1.20-bullseye
###
FROM ${GOLANG_BASE} as builder
ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
ARG BUILDFLAGS="-ldflags=-w -s"
ARG GOLICENSES_VERSION
ARG EP=/usr/local/bin/sgx-sw/intel-sgx-epchook
ARG CMD=sgx_epchook
ARG NFD_HOOK=intel-sgx-epchook
ARG SRC_DIR=/usr/local/bin/sgx-sw
WORKDIR ${DIR}
COPY . .
RUN (cd cmd/${CMD}; GO111MODULE=${GO111MODULE} CGO_ENABLED=0 go install "${BUILDFLAGS}") && install -D /go/bin/${CMD} /install_root${EP}
RUN install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && if [ ! -d "licenses/$CMD" ] ; then \
    GO111MODULE=on go run github.com/google/go-licenses@${GOLICENSES_VERSION} save "./cmd/$CMD" \
    --save_path /install_root/licenses/$CMD/go-licenses ; \
    else mkdir -p /install_root/licenses/$CMD/go-licenses/ && cd licenses/$CMD && cp -r * /install_root/licenses/$CMD/go-licenses/ ; fi
###
ARG TOYBOX_VERSION="0.8.10"
ARG TOYBOX_SHA256="3c31e235fe87e74e6c6cf7cd7299fcbffb0f4a4834dae607aa26bb4f1583549a"
ARG ROOT=/install_root
RUN apt-get update && apt-get --no-install-recommends -y install musl musl-tools musl-dev
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN curl -SL https://github.com/landley/toybox/archive/refs/tags/$TOYBOX_VERSION.tar.gz -o toybox.tar.gz \
    && echo "$TOYBOX_SHA256 toybox.tar.gz" | sha256sum -c - \
    && tar -xzf toybox.tar.gz \
    && rm toybox.tar.gz \
    && cd toybox-$TOYBOX_VERSION \
    && KCONFIG_CONFIG=${DIR}/build/docker/toybox-config LDFLAGS="--static" CC=musl-gcc PREFIX=$ROOT V=2 make toybox install \
    && install -D LICENSE $ROOT/licenses/toybox \
    && cp -r /usr/share/doc/musl $ROOT/licenses/
###
FROM ${FINAL_BASE}
LABEL vendor='Intel®'
LABEL version='devel'
LABEL release='1'
LABEL name='intel-sgx-initcontainer'
LABEL summary='Intel® SGX NFD hook for Kubernetes'
LABEL description='The SGX EPC memory available on each node is registered as a Kubernetes extended resource using node-feature-discovery (NFD). A custom NFD source hook is installed as part of SGX device plugin operator deployment and NFD is configured to register the SGX EPC memory extended resource reported by the hook'
COPY --from=builder /install_root /
ENTRYPOINT [ "/bin/sh", "-c", "cp -a /usr/local/bin/sgx-sw/intel-sgx-epchook /etc/kubernetes/node-feature-discovery/source.d/" ]
