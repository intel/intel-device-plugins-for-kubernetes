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

FROM debian:unstable-slim AS builder

RUN echo "deb-src http://deb.debian.org/debian unstable main" >> \
        /etc/apt/sources.list.d/deb-src.list && \
    apt update && apt install -y --no-install-recommends \
        gcc make patch autoconf automake libtool pkg-config \
        libjson-c-dev uuid-dev curl ca-certificates && \
    mkdir -p /usr/local/share/package-sources && \
    cd /usr/local/share/package-sources && \
    apt --download-only source uuid libjson-c5 && cd /

ARG ACCEL_CONFIG_VERSION="3.4.2"
ARG ACCEL_CONFIG_DOWNLOAD_URL="https://github.com/intel/idxd-config/archive/accel-config-v$ACCEL_CONFIG_VERSION.tar.gz"
ARG ACCEL_CONFIG_SHA256="9cb2151e86f83949a28f06a885be3bf3100906f9e3af667fa01b56e7666a3c1c"

RUN curl -fsSL "$ACCEL_CONFIG_DOWNLOAD_URL" -o accel-config.tar.gz && \
    echo "$ACCEL_CONFIG_SHA256 accel-config.tar.gz" | sha256sum -c - && \
    tar -xzf accel-config.tar.gz

ADD demo/idxd-config-kmod.patch /

RUN cd idxd-config-accel-config-v$ACCEL_CONFIG_VERSION && \
    patch -p1 < ../idxd-config-kmod.patch && \
    ./git-version-gen && \
    autoreconf -i && \
    ./configure -q --libdir=/usr/lib64 --disable-test --disable-docs && \
    make && \
    make install

FROM debian:unstable-slim

RUN apt update && apt install -y uuid libjson-c5 jq

COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 /lib/x86_64-linux-gnu/
RUN ldconfig

COPY --from=builder /usr/bin/accel-config /usr/bin/

COPY --from=builder /usr/local/share/package-sources/ \
    /usr/local/share/package-sources/
COPY --from=builder /accel-config.tar.gz /usr/local/share/package-sources/
COPY --from=builder /idxd-config-kmod.patch /usr/local/share/package-sources/

ADD demo/idxd-init.sh /idxd-init/
ADD demo/dsa.conf /idxd-init/

WORKDIR /idxd-init
ENTRYPOINT bash idxd-init.sh
