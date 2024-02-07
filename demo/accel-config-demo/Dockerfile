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

FROM debian:unstable-slim AS builder

ARG ACCEL_CONFIG_VERSION="4.1.5-1"

RUN apt-get update && apt-get install -y --no-install-recommends libaccel-config-dev=${ACCEL_CONFIG_VERSION} \
    gcc g++ nasm make cmake autoconf automake libtool pkg-config git ca-certificates uuid-dev

RUN cd / && git clone --recurse-submodules --branch v1.1.1 --depth 1 https://github.com/intel/DML.git && \
    mkdir DML/build && cd DML/build && \
    cmake .. && \
    make install && \
    mv -v /usr/local/bin/tests /usr/local/bin/dml_tests && \
    mv -v /usr/local/bin/tests_mt /usr/local/bin/dml_tests_mt

RUN cd / && git clone --recursive --depth 1 --branch v1.3.1 https://github.com/intel/qpl.git && \
    mkdir qpl/build && cd qpl/build && \
    cmake -DLOG_HW_INIT=ON .. && \
    make install

FROM debian:unstable-slim

ARG ACCEL_CONFIG_VERSION="4.1.5-1"
RUN apt-get update && apt-get install -y --no-install-recommends pciutils accel-config=${ACCEL_CONFIG_VERSION} accel-config-test=${ACCEL_CONFIG_VERSION} && rm -rf /var/lib/apt/lists/\*

RUN sed -i -e '442,450d;502,515d' /usr/libexec/accel-config/test/iaa_user_test_runner.sh
COPY --from=builder /usr/local /usr/local
COPY --from=builder /qpl/build/examples /usr/local/bin/

COPY dml-test.sh /usr/local/bin/
COPY qpl-test.sh /usr/local/bin/
