## This is a generated file, do not edit directly. Edit build/docker/templates/intel-idxd-config-initcontainer.Dockerfile.in instead.
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
FROM debian:unstable-slim AS builder
RUN apt-get update && apt-get install -y --no-install-recommends gcc make patch autoconf automake libtool pkg-config libjson-c-dev uuid-dev curl ca-certificates
ARG ACCEL_CONFIG_VERSION="3.5.0"
ARG ACCEL_CONFIG_DOWNLOAD_URL="https://github.com/intel/idxd-config/archive/accel-config-v$ACCEL_CONFIG_VERSION.tar.gz"
ARG ACCEL_CONFIG_SHA256="4d2fecbbb29f293791214f475c44e73c25171f75c1725dbc516731b768e2e7c9"
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN curl -fsSL "$ACCEL_CONFIG_DOWNLOAD_URL" -o accel-config.tar.gz && echo "$ACCEL_CONFIG_SHA256 accel-config.tar.gz" | sha256sum -c - && tar -xzf accel-config.tar.gz
RUN cd idxd-config-accel-config-v$ACCEL_CONFIG_VERSION && ./git-version-gen && autoreconf -i && ./configure -q --libdir=/usr/lib64 --disable-test --disable-docs && make && make install
###
FROM debian:unstable-slim
RUN apt-get update && apt-get install -y --no-install-recommends libjson-c5 jq && rm -rf /var/lib/apt/lists/\*
COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 "/lib/x86_64-linux-gnu/"
RUN ldconfig && mkdir -p /licenses/accel-config
COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /accel-config.tar.gz /licenses/accel-config/
COPY demo/idxd-init.sh /usr/local/bin/
COPY demo/dsa.conf /idxd-init/
COPY demo/iaa.conf /idxd-init/
RUN mkdir /idxd-init/scratch
WORKDIR /idxd-init
ENTRYPOINT ["bash", "/usr/local/bin/idxd-init.sh"]
