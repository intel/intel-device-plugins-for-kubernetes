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

FROM debian:unstable AS builder

RUN apt-get update && apt-get install -y \
            build-essential autoconf automake autotools-dev libtool \
            pkgconf asciidoc xmlto uuid-dev libjson-c-dev libkmod-dev \
            libudev-dev libkeyutils-dev curl

ARG ACCEL_CONFIG_VERSION="3.4.2"
ARG ACCEL_CONFIG_DOWNLOAD_URL="https://github.com/intel/idxd-config/archive/accel-config-v$ACCEL_CONFIG_VERSION.tar.gz"
ARG ACCEL_CONFIG_SHA256="9cb2151e86f83949a28f06a885be3bf3100906f9e3af667fa01b56e7666a3c1c"

RUN curl -fsSL "$ACCEL_CONFIG_DOWNLOAD_URL" -o accel-config.tar.gz \
    && echo "$ACCEL_CONFIG_SHA256 accel-config.tar.gz" | sha256sum -c - \
    && tar -xzf accel-config.tar.gz \
    && rm accel-config.tar.gz

RUN cd idxd-config-accel-config-v$ACCEL_CONFIG_VERSION && \
    mkdir m4 && \
    ./autogen.sh && \
    ./configure CFLAGS='-g -O2' --prefix=/usr --sysconfdir=/etc --libdir=/usr/lib64 --disable-test --disable-docs && \
    make && \
    make install

FROM debian:unstable-slim

RUN apt-get update && apt-get install -y uuid libjson-c5 kmod udev jq

COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 /lib/x86_64-linux-gnu/
RUN ldconfig

COPY --from=builder /usr/bin/accel-config /usr/bin/

ADD demo/idxd-init.sh /idxd-init/
ADD demo/dsa.conf /idxd-init/

WORKDIR /idxd-init
ENTRYPOINT bash idxd-init.sh
