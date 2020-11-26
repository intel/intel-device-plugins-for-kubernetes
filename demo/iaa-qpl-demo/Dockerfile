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

FROM fedora:32 AS builder

RUN dnf install -y accel-config accel-config-devel \
    git cmake make g++ nasm clang libuuid-devel

RUN git clone --recursive --depth 1 --branch v0.1.20 \
    https://github.com/intel/qpl.git qpl.git

RUN cd qpl.git && \
    mkdir build && \
    cd build && \
    cmake .. && \
    cmake --build . --target install

FROM fedora:32

RUN dnf install -y accel-config accel-config-devel python

COPY --from=builder /usr/local/bin/tests /usr/bin/iaa-tests
COPY --from=builder /usr/local/bin/test_frontend /usr/bin/iaa-test_frontend
COPY --from=builder /usr/local/bin/init_tests /usr/local/bin/iaa-init_tests

ENTRYPOINT /usr/bin/iaa-test_frontend && python init_tests.py --test_path=/usr/local/bin/iaa-init_tests
