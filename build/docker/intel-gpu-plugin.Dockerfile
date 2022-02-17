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

# GOLANG_BASE can be used to make the build reproducible by choosing an
# image by its hash:
# GOLANG_BASE=golang@sha256:9d64369fd3c633df71d7465d67d43f63bb31192193e671742fa1c26ebc3a6210
#
# This is used on release branches before tagging a stable version.
# The main branch defaults to using the latest Golang base image.
ARG GOLANG_BASE=golang:1.17-bullseye

FROM ${GOLANG_BASE} as builder

ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
ARG BUILDFLAGS="-ldflags=-w -s"
WORKDIR $DIR
COPY . .

RUN cd cmd/gpu_plugin; GO111MODULE=${GO111MODULE} CGO_ENABLED=0 go install "${BUILDFLAGS}"; cd -
RUN install -D /go/bin/gpu_plugin /install_root/usr/local/bin/intel_gpu_device_plugin \
    && install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && GO111MODULE=on go install github.com/google/go-licenses@v1.0.0 && go-licenses save "./cmd/gpu_plugin" --save_path /install_root/licenses/go-licenses

FROM gcr.io/distroless/static

LABEL name='intel-gpu-plugin' 
LABEL vendor='Intel®' 
LABEL version='devel' 
LABEL release='1' 
LABEL summary='Intel® GPU device plugin for Kubernetes' 
LABEL description='The GPU device plugin provides access to Intel discrete (Xe) and integrated GPU HW device files'

COPY --from=builder /install_root /
ENTRYPOINT ["/usr/local/bin/intel_gpu_device_plugin"]
