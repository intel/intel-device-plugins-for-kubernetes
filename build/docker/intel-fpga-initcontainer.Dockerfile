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

RUN cd cmd/fpga_crihook && GO111MODULE=${GO111MODULE} CGO_ENABLED=0 go install "${BUILDFLAGS}" && \
    cd ../fpga_tool && GO111MODULE=${GO111MODULE} CGO_ENABLED=0 go install "${BUILDFLAGS}" && \
    cd ../.. && \
    install -D ${DIR}/LICENSE $ROOT/usr/local/share/package-licenses/intel-device-plugins-for-kubernetes/LICENSE && \
    scripts/copy-modules-licenses.sh ./cmd/fpga_crihook $ROOT/usr/local/share/ && \
    scripts/copy-modules-licenses.sh ./cmd/fpga_tool $ROOT/usr/local/share/

ARG SRC_DIR=/usr/local/fpga-sw
ARG DST_DIR=/opt/intel/fpga-sw
ARG CRI_HOOK=intel-fpga-crihook

RUN install -D /go/bin/fpga_crihook $ROOT/$SRC_DIR/$CRI_HOOK
RUN install -D /go/bin/fpga_tool $ROOT/$SRC_DIR/

RUN echo "{\n\
    \"hook\" : \"$DST_DIR/$CRI_HOOK\",\n\
    \"stage\" : [ \"prestart\" ],\n\
    \"annotation\": [ \"fpga.intel.com/region\" ]\n\
}\n">>$ROOT/$SRC_DIR/$CRI_HOOK.json

ARG TOYBOX_VERSION="0.8.5"
ARG TOYBOX_SHA256="27cc073222f3b726ee10d96c4f32ac2c4c936b07ea195227736755971e6d90c9"
RUN apt update && apt -y install musl musl-tools musl-dev
RUN curl -SL https://github.com/landley/toybox/archive/refs/tags/$TOYBOX_VERSION.tar.gz -o toybox.tar.gz \
    && echo "$TOYBOX_SHA256 toybox.tar.gz" | sha256sum -c - \
    && tar -xzf toybox.tar.gz \
    && rm toybox.tar.gz \
    && cd toybox-$TOYBOX_VERSION \
    && KCONFIG_CONFIG=${DIR}/build/docker/toybox-config LDFLAGS="--static" CC=musl-gcc PREFIX=$ROOT V=2 make toybox install \
    && install -D LICENSE $ROOT/usr/local/share/package-licenses/toybox \
    && cp -r /usr/share/doc/musl $ROOT/usr/local/share/package-licenses/

FROM gcr.io/distroless/static
COPY --from=builder /install_root /

ENTRYPOINT [ "/bin/sh", "-c", "cp -a /usr/local/fpga-sw/* /opt/intel/fpga-sw/ && ln -sf /opt/intel/fpga-sw/intel-fpga-crihook.json /etc/containers/oci/hooks.d/" ]
