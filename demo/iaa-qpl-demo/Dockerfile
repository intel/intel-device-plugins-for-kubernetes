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
    gcc autoconf automake libtool pkg-config libjson-c-dev curl

RUN git clone --recursive --depth 1 --branch v0.1.20 \
    https://github.com/intel/qpl.git && \
    mkdir qpl/build && cd qpl/build && \
    cmake .. && \
    make install

ARG ACCEL_CONFIG_VERSION=v3.4.6.4

RUN curl -sSL https://github.com/intel/idxd-config/archive/accel-config-$ACCEL_CONFIG_VERSION.tar.gz | tar -zx && \
    cd idxd-config-accel-config-$ACCEL_CONFIG_VERSION && \
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
