# Copyright 2021-2022 Intel Corporation. All Rights Reserved.
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

FROM ubuntu:20.04 AS builder

RUN apt update && DEBIAN_FRONTEND=noninteractive TZ="Etc/UTC" \
    apt install -y --no-install-recommends \
    g++ clang nasm cmake make git ca-certificates uuid-dev \
    gcc autoconf automake libtool pkg-config libjson-c-dev curl zlib1g-dev

RUN git clone --recursive --depth 1 --branch v0.2.0 \
    https://github.com/intel/qpl.git && \
    mkdir qpl/build && cd qpl/build && \
    cmake .. && \
    make install

ARG ACCEL_CONFIG_VERSION=3.5.0
ARG ACCEL_CONFIG_DOWNLOAD_URL="https://github.com/intel/idxd-config/archive/accel-config-v$ACCEL_CONFIG_VERSION.tar.gz"
ARG ACCEL_CONFIG_SHA256="4d2fecbbb29f293791214f475c44e73c25171f75c1725dbc516731b768e2e7c9"

RUN curl -fsSL "$ACCEL_CONFIG_DOWNLOAD_URL" -o accel-config.tar.gz && echo "$ACCEL_CONFIG_SHA256 accel-config.tar.gz" | sha256sum -c - && tar -xzf accel-config.tar.gz
RUN cd idxd-config-accel-config-v$ACCEL_CONFIG_VERSION && \
    ./git-version-gen && \
    autoreconf -i && \
    ./configure -q --libdir=/usr/lib64 --disable-test --disable-docs && \
    make install

FROM ubuntu:20.04

RUN apt update && DEBIAN_FRONTEND=noninteractive TZ="Etc/UTC" \
    apt install -y libjson-c4 python

COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 "/lib/x86_64-linux-gnu/"
RUN ldconfig

COPY --from=builder /usr/local/bin/ /usr/local/bin/

ENTRYPOINT cd /usr/local/bin/test_frontend && python init_tests.py --test_path=/usr/local/bin/init_tests
