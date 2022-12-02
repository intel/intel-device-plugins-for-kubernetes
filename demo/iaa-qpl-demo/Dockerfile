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

FROM ubuntu:22.04 AS builder

RUN apt update && DEBIAN_FRONTEND=noninteractive TZ="Etc/UTC" \
    apt install -y --no-install-recommends \
    g++ clang nasm cmake make git ca-certificates uuid-dev \
    gcc autoconf automake libtool pkg-config libjson-c-dev curl zlib1g-dev

ARG ACCEL_CONFIG_VERSION=3.5.2
ARG ACCEL_CONFIG_DOWNLOAD_URL="https://github.com/intel/idxd-config/archive/accel-config-v$ACCEL_CONFIG_VERSION.tar.gz"
ARG ACCEL_CONFIG_SHA256="a2d52007b4bfdc050a21893466e78328c6800f9a87b0806c7e7f5775ff48387b"

RUN curl -fsSL "$ACCEL_CONFIG_DOWNLOAD_URL" -o accel-config.tar.gz && echo "$ACCEL_CONFIG_SHA256 accel-config.tar.gz" | sha256sum -c - && tar -xzf accel-config.tar.gz
RUN cd idxd-config-accel-config-v$ACCEL_CONFIG_VERSION && \
    ./git-version-gen && \
    autoreconf -i && \
    ./configure -q --libdir=/usr/lib64 --disable-test --disable-docs && \
    make install

RUN git clone --recursive --depth 1 --branch v1.0.0 \
    https://github.com/intel/qpl.git && \
    mkdir qpl/build && cd qpl/build && \
    sed -i '10i #include <stdexcept>' ../tools/benchmarks/include/types.hpp && \
    cmake -DLOG_HW_INIT=ON .. && \
    cmake --build . --target install

FROM ubuntu:22.04

RUN apt update && DEBIAN_FRONTEND=noninteractive TZ="Etc/UTC" \
    apt install -y libjson-c5 python3

COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 "/lib/x86_64-linux-gnu/"
RUN ldconfig

COPY --from=builder /usr/local/bin/ /usr/local/bin/

ENTRYPOINT cd /usr/local/bin/test_frontend && python init_tests.py --test_path=/usr/local/bin/init_tests
