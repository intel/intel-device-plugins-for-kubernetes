## This is a generated file, do not edit directly. Edit build/docker/templates/intel-gpu-levelzero.Dockerfile.in instead.
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
ARG CMD=gpu_levelzero
ARG BUILD_BASE=golang:1.25-trixie
FROM ${BUILD_BASE} AS builder
ARG DIR=/intel-device-plugins-for-kubernetes
ENV CGO_CFLAGS="-pipe -fno-plt"
ENV CGO_LDFLAGS="-fstack-protector-strong -Wl,-O1,--sort-common,--as-needed,-z,relro,-z,now,-z,noexecstack,-z,defs,-s,-w"
ENV CGOFLAGS="-trimpath -mod=readonly -buildmode=pie"
ENV GCFLAGS="all=-spectre=all -N -l"
ENV ASMFLAGS="all=-spectre=all"
ENV LDFLAGS="all=-linkmode=external -s -w"
ARG GOLICENSES_VERSION
ARG CMD
RUN mkdir /runtime
RUN apt-get update && apt-get install --no-install-recommends -y jq libc6-dev ocl-icd-libopencl1 gcc ca-certificates && \
    cd /runtime && \
    curl -sSLO https://github.com/intel/intel-graphics-compiler/releases/download/v2.20.3/intel-igc-core-2_2.20.3+19972_amd64.deb && \
    curl -sSLO https://github.com/intel/intel-graphics-compiler/releases/download/v2.20.3/intel-igc-opencl-2_2.20.3+19972_amd64.deb && \
    curl -sSLO https://github.com/intel/compute-runtime/releases/download/25.40.35563.4/intel-opencl-icd_25.40.35563.4-0_amd64.deb && \
    curl -sSLO https://github.com/intel/compute-runtime/releases/download/25.40.35563.4/libigdgmm12_22.8.2_amd64.deb && \
    curl -sSLO https://github.com/intel/compute-runtime/releases/download/25.40.35563.4/libze-intel-gpu1_25.40.35563.4-0_amd64.deb && \
    curl -sSLO https://github.com/oneapi-src/level-zero/releases/download/v1.24.3/level-zero_1.24.3+u22.04_amd64.deb && \
    curl -sSLO https://github.com/oneapi-src/level-zero/releases/download/v1.24.3/level-zero-devel_1.24.3+u22.04_amd64.deb && \
    dpkg -i *.deb && \
    rm -f *.deb && \
    rm -rf /var/lib/apt/lists/\*
ARG EP=/usr/local/bin/intel_gpu_levelzero
WORKDIR ${DIR}
COPY . .
RUN cd cmd/${CMD} && \
    GO111MODULE=on CGO_ENABLED=1 go install $CGOFLAGS --gcflags="$GCFLAGS" --asmflags="$ASMFLAGS" --ldflags="$LDFLAGS" && \
    install -D /go/bin/${CMD} /install_root${EP}
RUN install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && if [ ! -d "licenses/$CMD" ] ; then \
    GO111MODULE=on GOROOT=$(go env GOROOT) go run github.com/google/go-licenses@${GOLICENSES_VERSION} save "./cmd/$CMD" \
    --save_path /install_root/licenses/$CMD/go-licenses ; \
    else mkdir -p /install_root/licenses/$CMD/go-licenses/ && cd licenses/$CMD && cp -r * /install_root/licenses/$CMD/go-licenses/ ; fi && \
    echo "Verifying installed licenses" && test -e /install_root/licenses/$CMD/go-licenses
FROM debian:unstable-slim
ARG CMD
COPY --from=builder /runtime /runtime
RUN apt-get update && apt-get install --no-install-recommends -y ocl-icd-libopencl1 curl ca-certificates && \
    cd /runtime && \
    curl -sSLO https://github.com/intel/intel-graphics-compiler/releases/download/v2.20.3/intel-igc-core-2_2.20.3+19972_amd64.deb && \
    curl -sSLO https://github.com/intel/intel-graphics-compiler/releases/download/v2.20.3/intel-igc-opencl-2_2.20.3+19972_amd64.deb && \
    curl -sSLO https://github.com/intel/compute-runtime/releases/download/25.40.35563.4/intel-opencl-icd_25.40.35563.4-0_amd64.deb && \
    curl -sSLO https://github.com/intel/compute-runtime/releases/download/25.40.35563.4/libigdgmm12_22.8.2_amd64.deb && \
    curl -sSLO https://github.com/intel/compute-runtime/releases/download/25.40.35563.4/libze-intel-gpu1_25.40.35563.4-0_amd64.deb && \
    curl -sSLO https://github.com/oneapi-src/level-zero/releases/download/v1.24.3/level-zero_1.24.3+u22.04_amd64.deb && \
    dpkg -i *.deb && \
    apt-get -y remove ca-certificates && \
    apt-get -y autoremove && \
    rm -f *.deb && \
    rm -rf /var/lib/apt/lists/\* && \
    rm "/lib/x86_64-linux-gnu/libze_validation"* && rm "/lib/x86_64-linux-gnu/libze_tracing_layer"*
COPY --from=builder /install_root /
ENTRYPOINT ["/usr/local/bin/intel_gpu_levelzero"]
LABEL vendor='Intel®'
LABEL org.opencontainers.image.source='https://github.com/intel/intel-device-plugins-for-kubernetes'
LABEL maintainer="Intel®"
LABEL version='0.35.0'
LABEL release='1'
LABEL name='intel-gpu-levelzero'
LABEL summary='Intel® GPU levelzero for Kubernetes'
LABEL description='The GPU levelzero container provides access to Levelzero API for the Intel GPU plugin'
