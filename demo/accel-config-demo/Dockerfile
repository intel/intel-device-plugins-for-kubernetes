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

FROM ubuntu:22.04 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc g++ nasm make cmake patch autoconf automake libtool pkg-config git curl ca-certificates \
    libjson-c-dev uuid-dev zlib1g-dev libssl-dev

ARG ACCEL_CONFIG_VERSION="4.0"
ARG ACCEL_CONFIG_DOWNLOAD_URL="https://github.com/intel/idxd-config/archive/accel-config-v$ACCEL_CONFIG_VERSION.tar.gz"
ARG ACCEL_CONFIG_SHA256="6d9bce40d797a87508ce5c5a5b71bb2bdc1cb544848925ed2c4c37b46a1252ce"

RUN curl -fsSL "$ACCEL_CONFIG_DOWNLOAD_URL" -o accel-config.tar.gz && echo "$ACCEL_CONFIG_SHA256 accel-config.tar.gz" | sha256sum -c - && tar -xzf accel-config.tar.gz

COPY idxd-reset.patch /
COPY test_runner_disable_shared_queues.patch /

RUN cd idxd-config-accel-config-v$ACCEL_CONFIG_VERSION && \
    patch -p1 < ../idxd-reset.patch && \
    patch -p1 < ../test_runner_disable_shared_queues.patch && \
    ./git-version-gen && \
    autoreconf -i && \
    ./configure -q --libdir=/usr/lib64 --enable-test=yes --disable-docs && \
    make install

COPY dml.patch /
RUN cd / && git clone --recurse-submodules --branch v0.1.9-beta --depth 1 https://github.com/intel/DML.git && \
    mkdir DML/build && cd DML/build && \
    patch -d .. -p1 < /dml.patch && \
    cmake .. && \
    make install && \
    mv -v /usr/local/bin/tests /usr/local/bin/dml_tests && \
    mv -v /usr/local/bin/tests_mt /usr/local/bin/dml_tests_mt && \
    mv -v /usr/local/bin/job_api_samples /usr/local/bin/dml_job_api_samples

RUN cd / && git clone --recursive --depth 1 --branch v1.0.0 https://github.com/intel/qpl.git && \
    mkdir qpl/build && cd qpl/build && \
    sed -i '10i #include <stdexcept>' ../tools/benchmarks/include/types.hpp && \
    cmake -DLOG_HW_INIT=ON .. && \
    make install

FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends libjson-c5 libssl3 && rm -rf /var/lib/apt/lists/\*

COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 "/lib/x86_64-linux-gnu/"
RUN ldconfig

COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /usr/libexec/accel-config/test /test
COPY --from=builder /idxd-reset.patch /usr/local/share/package-sources/
COPY --from=builder /test_runner_disable_shared_queues.patch /usr/local/share/package-sources/

COPY --from=builder /usr/local /usr/local
COPY --from=builder /qpl/build/examples /usr/local/bin/

COPY dml-test.sh /usr/local/bin/
COPY qpl-test.sh /usr/local/bin/

ENTRYPOINT cd /test && /bin/bash -e ./dsa_user_test_runner.sh
