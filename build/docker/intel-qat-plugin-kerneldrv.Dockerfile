## This is a generated file, do not edit directly. Edit build/docker/templates/intel-qat-plugin-kerneldrv.Dockerfile.in instead.
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
ARG GOLANG_BASE=golang:1.23-bookworm
###
FROM ${GOLANG_BASE} AS builder
ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
ARG LDFLAGS="all=-w -s"
ARG GOFLAGS="-trimpath"
ARG GCFLAGS="all=-spectre=all -N -l"
ARG ASMFLAGS="all=-spectre=all"
ARG GOLICENSES_VERSION
ARG EP=/usr/local/bin/intel_sgx_device_plugin
ARG CMD=qat_plugin
WORKDIR $DIR
COPY . .
ARG QAT_DRIVER_RELEASE="qat1.7.l.4.14.0-00031"
ARG QAT_DRIVER_SHA256="a68dfaea4308e0bb5f350b7528f1a076a0c6ba3ec577d60d99dc42c49307b76e"
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN mkdir -p /usr/src/qat && cd /usr/src/qat && wget -q https://downloadmirror.intel.com/30178/eng/$QAT_DRIVER_RELEASE.tar.gz     && echo "$QAT_DRIVER_SHA256 $QAT_DRIVER_RELEASE.tar.gz" | sha256sum -c -     && tar xf *.tar.gz     && cd /usr/src/qat/quickassist/utilities/adf_ctl     && LDFLAGS= make KERNEL_SOURCE_DIR=/usr/src/qat/quickassist/qat     && install -D adf_ctl /install_root/usr/local/bin/adf_ctl
RUN (cd cmd/$CMD && GOFLAGS=${GOFLAGS} GO111MODULE=${GO111MODULE} CGO_ENABLED=1 go install -gcflags="${GCFLAGS}" -asmflags="${ASMFLAGS}" -ldflags="${LDFLAGS}" -tags kerneldrv)
RUN chmod a+x /go/bin/$CMD && install -D /go/bin/$CMD /install_root/usr/local/bin/intel_qat_device_plugin
RUN install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && if [ ! -d "licenses/$CMD" ] ; then \
    GO111MODULE=on go run github.com/google/go-licenses@${GOLICENSES_VERSION} save "./cmd/$CMD" \
    --save_path /install_root/licenses/$CMD/go-licenses ; \
    else mkdir -p /install_root/licenses/$CMD/go-licenses/ && cd licenses/$CMD && cp -r * /install_root/licenses/$CMD/go-licenses/ ; fi
FROM debian:unstable-slim
LABEL vendor='Intel®'
LABEL version='devel'
LABEL release='1'
LABEL name='intel-qat-plugin-kerneldrv'
LABEL summary='Intel® QAT device plugin kerneldrv for Kubernetes'
COPY --from=builder /install_root /
ENV PATH=/usr/local/bin
ENTRYPOINT ["/usr/local/bin/intel_qat_device_plugin"]
