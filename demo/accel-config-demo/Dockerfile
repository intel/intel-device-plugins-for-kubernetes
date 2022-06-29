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

FROM ubuntu:20.04 AS builder

RUN apt update && apt install -y --no-install-recommends \
    gcc make patch autoconf automake libtool pkg-config curl ca-certificates \
    libjson-c-dev uuid-dev

ARG ACCEL_CONFIG_VERSION=v3.4.6.4

RUN curl -sSL https://github.com/intel/idxd-config/archive/accel-config-$ACCEL_CONFIG_VERSION.tar.gz | tar -zx

ADD idxd-reset.patch /
ADD test_runner_disable_shared_queues.patch /

RUN cd idxd-config-accel-config-$ACCEL_CONFIG_VERSION && \
    patch -p1 < ../idxd-reset.patch && \
    patch -p1 < ../test_runner_disable_shared_queues.patch && \
    ./git-version-gen && \
    autoreconf -i && \
    ./configure -q --libdir=/usr/lib64 --enable-test=yes --disable-docs && \
    make install

FROM ubuntu:20.04

RUN apt update && apt install -y libjson-c4

COPY --from=builder /usr/lib64/libaccel-config.so.1.0.0 "/lib/x86_64-linux-gnu/"
RUN ldconfig

COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /usr/share/accel-config/test /test
COPY --from=builder /idxd-reset.patch /usr/local/share/package-sources/

ENTRYPOINT cd /test && /bin/bash -e ./dsa_user_test_runner.sh
