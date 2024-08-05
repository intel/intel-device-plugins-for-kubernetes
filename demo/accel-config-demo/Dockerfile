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

FROM debian:unstable-slim

RUN apt-get update && apt-get install -y --no-install-recommends pciutils accel-config accel-config-test && rm -rf /var/lib/apt/lists/\*

RUN sed -i -e '/(IAA_OPCODE_MASK_DECOMPRESS\|(IAA_OPCODE_MASK_TRANSL_FETCH/,+9d' /usr/libexec/accel-config/test/iaa_user_test_runner.sh
