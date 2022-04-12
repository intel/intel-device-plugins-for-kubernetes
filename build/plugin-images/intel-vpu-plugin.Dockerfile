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
ARG GOLANG_BASE=golang:1.18-bullseye

# FINAL_BASE can be used to configure the base image of the final image.
#
# This is used in two ways:
# 1) make <image-name> BUILDER=<docker|buildah>
# 2) docker build ... -f <image-name>.Dockerfile
#
# The project default is 1) which sets FINAL_BASE=gcr.io/distroless/static
# (see build-image.sh).
# Declaring FINAL_BASE ARG but not setting the value to resolve build warning:
# "[Warning] one or more build args were not consumed: [FINAL_BASE]"
ARG FINAL_BASE

FROM ${GOLANG_BASE} as builder

ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
ARG BUILDFLAGS="-ldflags=-w -s"
WORKDIR $DIR
COPY . .

RUN echo "deb-src http://deb.debian.org/debian unstable main" | tee -a /etc/apt/sources.list
RUN apt update && apt -y install dpkg-dev libusb-1.0-0-dev
RUN mkdir -p /install_root/licenses/libusb \
    && cd /install_root/licenses/libusb \
    && apt-get --download-only source libusb-1.0-0 \
    && cd -
RUN cd cmd/vpu_plugin; GO111MODULE=${GO111MODULE} CGO_ENABLED=1 go install "${BUILDFLAGS}"; cd -
RUN install -D /go/bin/vpu_plugin /install_root/usr/local/bin/intel_vpu_device_plugin \
    && install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && GO111MODULE=on go install github.com/google/go-licenses@v1.0.0 && go-licenses save "./cmd/vpu_plugin" --save_path /install_root/licenses/go-licenses

FROM debian:unstable-slim
RUN apt update && apt -y install libusb-1.0-0
COPY --from=builder /install_root /
ENTRYPOINT ["/usr/local/bin/intel_vpu_device_plugin"]
