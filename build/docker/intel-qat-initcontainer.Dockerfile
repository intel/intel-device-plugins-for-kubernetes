## This is a generated file, do not edit directly. Edit build/docker/templates/intel-qat-initcontainer.Dockerfile.in instead.
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
ARG FINAL_BASE=registry.access.redhat.com/ubi9-micro:latest
###
##
## GOLANG_BASE can be used to make the build reproducible by choosing an
## image by its hash:
## GOLANG_BASE=golang@sha256:9d64369fd3c633df71d7465d67d43f63bb31192193e671742fa1c26ebc3a6210
##
## This is used on release branches before tagging a stable version.
## The main branch defaults to using the latest Golang base image.
ARG GOLANG_BASE=golang:1.23-bookworm
###
FROM ${GOLANG_BASE} AS builder
ARG DIR=/intel-device-plugins-for-kubernetes
WORKDIR $DIR
COPY . .
RUN install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE
ARG TOYBOX_VERSION="0.8.12"
ARG TOYBOX_SHA256="3c529d93923dde67d048e7bcbd5d1bc0dd1ad09362269e2415f5f2eaab349b5b"
ARG ROOT=/install_root
RUN apt-get update && apt-get --no-install-recommends -y install musl musl-tools musl-dev
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG FINAL_BASE=registry.access.redhat.com/ubi9-micro:latest
RUN curl -SL https://github.com/landley/toybox/archive/refs/tags/$TOYBOX_VERSION.tar.gz -o toybox.tar.gz \
    && echo "$TOYBOX_SHA256 toybox.tar.gz" | sha256sum -c - \
    && tar -xzf toybox.tar.gz \
    && rm toybox.tar.gz \
    && cd toybox-$TOYBOX_VERSION \
    && KCONFIG_CONFIG=${DIR}/build/docker/toybox-config-$(echo ${FINAL_BASE} | xargs basename -s :latest) LDFLAGS="--static" CC=musl-gcc PREFIX=$ROOT/usr/bin V=2 make toybox install_flat \
    && install -D LICENSE $ROOT/licenses/toybox \
    && cp -r /usr/share/doc/musl $ROOT/licenses/
###
FROM ${FINAL_BASE}
LABEL vendor='Intel®'
LABEL version='0.32.1'
LABEL maintainer="Intel®"
LABEL release='1'
LABEL name='intel-qat-initcontainer'
LABEL summary='Intel® QAT initcontainer for Kubernetes'
LABEL description='Intel QAT initcontainer initializes devices'
COPY --from=builder /install_root /
COPY demo/qat-init.sh /usr/local/bin/
COPY demo/qat-autoreset.sh /usr/local/bin/
WORKDIR /qat-init
ENTRYPOINT ["bash", "-c", "/usr/local/bin/qat-init.sh && /usr/local/bin/qat-autoreset.sh"]
