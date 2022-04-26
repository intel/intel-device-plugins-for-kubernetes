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

# FINAL_BASE can be used to configure the base image of the final image.
#
# This is used in two ways:
# 1) make <image-name> BUILDER=<docker|buildah>
# 2) docker build ... -f <image-name>.Dockerfile
#
# The project default is 1) which sets FINAL_BASE=gcr.io/distroless/static
# (see build-image.sh).
# Declaring FINAL_BASE ARG but not setting the value to resolve build warning:
# "[Warning] one or more build args were not consumed: [FINAL_BASE]"
ARG FINAL_BASE

FROM debian:unstable-slim AS builder

RUN echo "deb-src http://deb.debian.org/debian unstable main" >> \
        /etc/apt/sources.list.d/deb-src.list && \
    apt update && apt install -y --no-install-recommends \
        gcc make patch autoconf automake libtool pkg-config \
        libjson-c-dev uuid-dev curl ca-certificates

ARG ACCEL_CONFIG_VERSION="3.4.6.3"
ARG ACCEL_CONFIG_DOWNLOAD_URL="https://github.com/intel/idxd-config/archive/accel-config-v$ACCEL_CONFIG_VERSION.tar.gz"
ARG ACCEL_CONFIG_SHA256="a28f531dd69bdc83ca2ad23cacd079530510e98b726421c6d07e24c8426d086e"

RUN curl -fsSL "$ACCEL_CONFIG_DOWNLOAD_URL" -o accel-config.tar.gz && \
    echo "$ACCEL_CONFIG_SHA256 accel-config.tar.gz" | sha256sum -c - && \
    tar -xzf accel-config.tar.gz

RUN cd idxd-config-accel-config-v$ACCEL_CONFIG_VERSION && \
    ./git-version-gen && \
    autoreconf -i && \
    ./configure -q --libdir=/usr/lib64 --disable-test --disable-docs && \
    make && \
    make install

FROM debian:unstable-slim

RUN apt update && apt install -y libjson-c5 jq

COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 /lib/x86_64-linux-gnu/
RUN ldconfig && mkdir -p /licenses/accel-config

COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /accel-config.tar.gz /licenses/accel-config/

ADD demo/idxd-init.sh /idxd-init/
ADD demo/dsa.conf /idxd-init/

WORKDIR /idxd-init
ENTRYPOINT bash idxd-init.sh
