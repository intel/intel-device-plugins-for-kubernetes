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
ARG ROCKYLINUX=1
## FINAL_BASE_DYN can be used to configure the base image of the final image.
## The project default is 1) which sets FINAL_BASE_DYN=gcr.io/distroless/cc-debian12
## (see build-image.sh).
## 2) and the default FINAL_BASE is primarily used to build Redhat Certified Openshift Operator container images that must be UBI based.
## The RedHat build tool does not allow additional image build parameters.
ARG BUILD_BASE=rockylinux:9
ARG FINAL_BASE_DYN=registry.access.redhat.com/ubi9/ubi-minimal:9.3
###
## Use the BUILD_BASE when either the a) golang-trixie is updated to a newer glibc
## or b) the intel-igc-core libraries are fixed to not to demand a newer glibc
FROM ${FINAL_BASE_DYN} AS builder
ARG DIR=/intel-device-plugins-for-kubernetes
ENV CGO_CFLAGS="-pipe -fno-plt"
ENV CGO_LDFLAGS="-fstack-protector-strong -Wl,-O1,--sort-common,--as-needed,-z,relro,-z,now,-z,noexecstack,-z,defs,-s,-w"
ENV CGOFLAGS="-trimpath -mod=readonly -buildmode=pie"
ENV GCFLAGS="all=-spectre=all -N -l"
ENV ASMFLAGS="all=-spectre=all"
ENV LDFLAGS="all=-linkmode=external -s -w"
ARG GOLICENSES_VERSION
ARG CMD
ARG ROCKYLINUX
ARG CGO_VERSION=1.25
RUN mkdir /runtime
RUN if [ $ROCKYLINUX -eq 0 ]; then \
        apt-get update && apt-get install --no-install-recommends -y wget jq curl libc6-dev ocl-icd-libopencl1 gcc ca-certificates && \
        LATEST_GO=$(curl --no-progress-meter https://go.dev/dl/?mode=json | jq ".[] | select(.version | startswith(\"go${CGO_VERSION}\")).version" | tr -d "\"") && \
        wget -q https://go.dev/dl/$LATEST_GO.linux-amd64.tar.gz -O - | tar -xz -C /usr/local && \
        cd /runtime && \
        wget -q https://github.com/intel/compute-runtime/releases/download/25.09.32961.7/intel-level-zero-gpu_1.6.32961.7_amd64.deb && \
        wget -q https://github.com/intel/compute-runtime/releases/download/25.09.32961.7/intel-opencl-icd_25.09.32961.7_amd64.deb && \
        wget -q https://github.com/intel/compute-runtime/releases/download/25.09.32961.7/libigdgmm12_22.6.0_amd64.deb && \
        wget -q https://github.com/oneapi-src/level-zero/releases/download/v1.20.2/level-zero-devel_1.20.2+u22.04_amd64.deb && \
        wget -q https://github.com/oneapi-src/level-zero/releases/download/v1.20.2/level-zero_1.20.2+u22.04_amd64.deb && \
        wget -q https://github.com/intel/intel-graphics-compiler/releases/download/v2.8.3/intel-igc-core-2_2.8.3+18762_amd64.deb && \
        wget -q https://github.com/intel/intel-graphics-compiler/releases/download/v2.8.3/intel-igc-opencl-2_2.8.3+18762_amd64.deb && \
        dpkg -i *.deb && \
        rm -rf /var/lib/apt/lists/\*; \
    else \
        source /etc/os-release && dnf install -y gcc jq wget 'dnf-command(config-manager)' && \
        dnf config-manager --add-repo https://repositories.intel.com/gpu/rhel/${VERSION_ID}/lts/2350/unified/intel-gpu-${VERSION_ID}.repo && \
        dnf install -y intel-opencl level-zero level-zero-devel intel-level-zero-gpu intel-gmmlib intel-ocloc && \
        dnf clean all && \
        LATEST_GO=$(curl --no-progress-meter https://go.dev/dl/?mode=json | jq ".[] | select(.version | startswith(\"go${CGO_VERSION}\")).version" | tr -d "\"") && \
        wget -q https://go.dev/dl/$LATEST_GO.linux-amd64.tar.gz -O - | tar -xz -C /usr/local && \
        cp -a /etc/OpenCL /usr/lib64/libocloc.so /usr/lib64/libze_intel_gpu.* /usr/lib64/libze_loader.* /usr/lib64/libigdgmm.* /runtime/ && \
        mkdir /runtime/licenses/ && cd /usr/share/licenses/ && cp -a level-zero intel-gmmlib intel-level-zero-gpu intel-ocloc /runtime/licenses/; \
    fi
ARG EP=/usr/local/bin/intel_gpu_levelzero
ARG CMD
WORKDIR ${DIR}
COPY . .
## Apply for the build phase as well as the license copy below the build.
ENV PATH=$PATH:/usr/local/go/bin/
RUN cd cmd/${CMD} && \
    GO111MODULE=on CGO_ENABLED=1 go install $CGOFLAGS --gcflags="$GCFLAGS" --asmflags="$ASMFLAGS" --ldflags="$LDFLAGS"
RUN [ $ROCKYLINUX -eq 0 ] && install -D /go/bin/${CMD} /install_root${EP} || install -D /root/go/bin/${CMD} /install_root${EP}
RUN install -D ${DIR}/LICENSE /install_root/licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && if [ ! -d "licenses/$CMD" ] ; then \
    GO111MODULE=on GOROOT=$(go env GOROOT) go run github.com/google/go-licenses@${GOLICENSES_VERSION} save "./cmd/$CMD" \
    --save_path /install_root/licenses/$CMD/go-licenses ; \
    else mkdir -p /install_root/licenses/$CMD/go-licenses/ && cd licenses/$CMD && cp -r * /install_root/licenses/$CMD/go-licenses/ ; fi && \
    echo "Verifying installed licenses" && test -e /install_root/licenses/$CMD/go-licenses
FROM ${FINAL_BASE_DYN}
ARG CMD
ARG ROCKYLINUX
COPY --from=builder /runtime /runtime
RUN if [ $ROCKYLINUX -eq 0 ]; then \
        apt-get update && apt-get install --no-install-recommends -y ocl-icd-libopencl1 && \
        rm /runtime/level-zero-devel_*.deb && \
        cd /runtime && dpkg -i *.deb && rm -rf /runtime && \
        rm "/lib/x86_64-linux-gnu/libze_validation"* && rm "/lib/x86_64-linux-gnu/libze_tracing_layer"*; \
    else \
        cp -a /runtime//*.so* /usr/lib64/ && cp -a /runtime/OpenCL /etc/ && cp -a /runtime/licenses/* /usr/share/licenses/; \
    fi
COPY --from=builder /install_root /
ENTRYPOINT ["/usr/local/bin/intel_gpu_levelzero"]
LABEL vendor='Intel®'
LABEL org.opencontainers.image.source='https://github.com/intel/intel-device-plugins-for-kubernetes'
LABEL maintainer="Intel®"
LABEL version='devel'
LABEL release='1'
LABEL name='intel-gpu-levelzero'
LABEL summary='Intel® GPU levelzero for Kubernetes'
LABEL description='The GPU levelzero container provides access to Levelzero API for the Intel GPU plugin'
