# Copyright 2022 Intel Corporation. All Rights Reserved.
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
ARG GOLANG_BASE=golang:1.17-bullseye

# FINAL_BASE can be used to configure the base image of the final image.
#
# This is used in two ways:
# 1) make <image-name> BUILDER=<docker|buildah>
# 2) docker build ... -f <image-name>.Dockerfile
#
# The project default is 1) which sets FINAL_BASE=gcr.io/distroless/static
# (see build-image.sh).
ARG FINAL_BASE=registry.access.redhat.com/ubi8-micro

FROM ${GOLANG_BASE} as builder

ARG DIR=/intel-device-plugins-for-kubernetes
WORKDIR $DIR
COPY . .

ARG ROOT=/install_root
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

FROM ${FINAL_BASE}

LABEL name='intel-qat-initcontainer'
LABEL vendor='Intel®'
LABEL version='devel'
LABEL release='1'
LABEL summary='Intel® QAT initcontainer for Kubernetes'
LABEL description='Intel QAT initcontainer initializes devices'

COPY --from=builder /install_root /

ADD demo/qat-init.sh /qat-init/

ENTRYPOINT /qat-init/qat-init.sh