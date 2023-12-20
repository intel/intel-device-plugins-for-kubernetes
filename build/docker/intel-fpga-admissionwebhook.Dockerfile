## This is a generated file, do not edit directly. Edit build/docker/templates/intel-fpga-admissionwebhook.Dockerfile.in instead.
##
## Copyright 2022 Intel Corporation. All Rights Reserved.
##
## Licensed under the Apache License, Version 2.0 (the "License");
## you may not use this file except in compliance with the License.
## You may obtain a copy of the License at
##
## http://www.apache.org/licenses/LICENSE-2.0
##
## Unless required by applicable law or agreed to in writing, software
## distributed under the License is distributed on an "AS IS" BASIS,
## WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
## See the License for the specific language governing permissions and
## limitations under the License.
###
ARG CMD=fpga_admissionwebhook
## FINAL_BASE can be used to configure the base image of the final image.
##
## This is used in two ways:
## 1) make <image-name> BUILDER=<docker|buildah>
## 2) docker build ... -f <image-name>.Dockerfile
##
## The project default is 1) which sets FINAL_BASE=gcr.io/distroless/static
## (see build-image.sh).
## 2) and the default FINAL_BASE is primarily used to build Redhat Certified Openshift Operator container images that must be UBI based.
## The RedHat build tool does not allow additional image build parameters.
ARG FINAL_BASE=registry.access.redhat.com/ubi9-micro:latest
###
##
## GOLANG_BASE can be used to make the build reproducible by choosing an
## image by its hash:
## GOLANG_BASE=golang@sha256:9d64369fd3c633df71d7465d67d43f63bb31192193e671742fa1c26ebc3a6210
##
## This is used on release branches before tagging a stable version.
## The main branch defaults to using the latest Golang base image.
ARG GOLANG_BASE=golang:1.21-bookworm
###
FROM ${GOLANG_BASE} as builder
ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
ARG LDFLAGS="-ldflags=all=-w -s"
ARG GOFLAGS=-trimpath
ARG GCFLAGS="-gcflags=all=-spectre=all -N -l"
ARG ASMFLAGS="-asmflags=all=-spectre=all"
ARG GOLICENSES_VERSION
ARG EP=/usr/local/bin/intel_fpga_admissionwebhook
ARG CMD
WORKDIR ${DIR}
COPY . .
RUN (cd cmd/${CMD}; GO111MODULE=${GO111MODULE} GOFLAGS=${GOFLAGS} CGO_ENABLED=0 go install "${GCFLAGS}" "${ASMFLAGS}" "${LDFLAGS}") && install -D /go/bin/${CMD} /install_root${EP}
RUN install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && if [ ! -d "licenses/$CMD" ] ; then \
    GO111MODULE=on go run github.com/google/go-licenses@${GOLICENSES_VERSION} save "./cmd/$CMD" \
    --save_path /install_root/licenses/$CMD/go-licenses ; \
    else mkdir -p /install_root/licenses/$CMD/go-licenses/ && cd licenses/$CMD && cp -r * /install_root/licenses/$CMD/go-licenses/ ; fi
###
FROM ${FINAL_BASE}
COPY --from=builder /install_root /
ENTRYPOINT ["/usr/local/bin/intel_fpga_admissionwebhook"]
LABEL vendor='Intel®'
LABEL version='0.29.0'
LABEL release='1'
LABEL name='intel-fpga-admissionwebhook'
LABEL summary='Intel® FPGA admission controller webhook for Kubernetes'
LABEL description='The FPGA admission controller webhook is responsible for performing mapping from user-friendly function IDs to the Interface ID and Bitstream ID that are required for FPGA programming. It also implements access control by namespacing FPGA configuration information'
