# Copyright 2021 Intel Corporation. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# GOLANG_BASE can be used to make the build reproducible by choosing an
# image by its hash:
# GOLANG_BASE=golang@sha256:9d64369fd3c633df71d7465d67d43f63bb31192193e671742fa1c26ebc3a6210
#
# This is used on release branches before tagging a stable version.
# The main branch defaults to using the latest Golang base image.
ARG GOLANG_BASE=golang:1.17-bullseye

FROM ${GOLANG_BASE} as builder

ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
ARG BUILDFLAGS="-ldflags=-w -s"
WORKDIR $DIR
COPY . .

ARG ROOT=/install_root

# Build NFD Feature Detector Hook
RUN cd cmd/gpu_nfdhook && GO111MODULE=${GO111MODULE} CGO_ENABLED=0 go install "${BUILDFLAGS}" && cd -\
    install -D ${DIR}/LICENSE $ROOT/licenses/intel-device-plugins-for-kubernetes/LICENSE && \
    mkdir -p  $ROOT/usr/local/share/ && \
    GO111MODULE=on go install github.com/google/go-licenses@v1.0.0 && go-licenses save "./cmd/gpu_nfdhook" --save_path $ROOT/licenses/go-licenses

ARG NFD_HOOK=intel-gpu-nfdhook
ARG SRC_DIR=/usr/local/bin/gpu-sw

RUN install -D /go/bin/gpu_nfdhook $ROOT/$SRC_DIR/$NFD_HOOK

ARG TOYBOX_VERSION="0.8.6"
ARG TOYBOX_SHA256="e2c4f72a158581a12f4303d0d1aeec196b01f293e495e535bcdaf75eb9ae0987"

RUN apt update && apt -y install musl musl-tools musl-dev
RUN curl -SL https://github.com/landley/toybox/archive/refs/tags/$TOYBOX_VERSION.tar.gz -o toybox.tar.gz \
    && echo "$TOYBOX_SHA256 toybox.tar.gz" | sha256sum -c - \
    && tar -xzf toybox.tar.gz \
    && rm toybox.tar.gz \
    && cd toybox-$TOYBOX_VERSION \
    && KCONFIG_CONFIG=${DIR}/build/docker/toybox-config LDFLAGS="--static" CC=musl-gcc PREFIX=$ROOT V=2 make toybox install \
    && install -D LICENSE $ROOT/licenses/toybox \
    && cp -r /usr/share/doc/musl $ROOT/licenses/

FROM gcr.io/distroless/static

LABEL name='intel-gpu-initcontainer' 
LABEL vendor='Intel®' 
LABEL version='devel' 
LABEL release='1' 
LABEL summary='Intel® GPU NFD hook for Kubernetes' 
LABEL description='The GPU fractional resources, such as GPU memory is registered as a kubernetes extended resource using node-feature-discovery (NFD). A custom NFD source hook is installed as part of GPU device plugin operator deployment and NFD is configured to register the GPU memory extended resource reported by the hook'

COPY --from=builder /install_root /

ENTRYPOINT [ "/bin/sh", "-c", "cp -a /usr/local/bin/gpu-sw/intel-gpu-nfdhook /etc/kubernetes/node-feature-discovery/source.d/" ]
