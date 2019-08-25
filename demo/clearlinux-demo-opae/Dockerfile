# CLEAR_LINUX_BASE and CLEAR_LINUX_VERSION can be used to make the build
# reproducible by choosing an image by its hash and installing an OS version
# with --version=:
# CLEAR_LINUX_BASE=clearlinux@sha256:b8e5d3b2576eb6d868f8d52e401f678c873264d349e469637f98ee2adf7b33d4
# CLEAR_LINUX_VERSION="--version=29970"
#
# This is used on release branches before tagging a stable version.
# The master branch defaults to using the latest Clear Linux.
ARG CLEAR_LINUX_BASE=clearlinux:latest

FROM ${CLEAR_LINUX_BASE} as builder

ARG CLEAR_LINUX_VERSION=

RUN swupd update --no-boot-update ${CLEAR_LINUX_VERSION}

# Fetch dependencies and source code
ARG OPAE_RElEASE=1.3.2-1

RUN swupd bundle-add wget c-basic devpkg-json-c devpkg-util-linux devpkg-hwloc doxygen Sphinx && \
    mkdir -p /usr/src/opae && \
    cd /usr/src/opae && \
    wget https://github.com/OPAE/opae-sdk/archive/${OPAE_RElEASE}.tar.gz && \
    tar xf *.tar.gz

# Build OPAE
RUN cd /usr/src/opae/opae-sdk-${OPAE_RElEASE} && \
    mkdir build && \
    cd build && \
    cmake .. -DBUILD_ASE=0 -DCMAKE_INSTALL_PREFIX=/usr -DCMAKE_SKIP_RPATH=true && \
    make xfpga nlb0 nlb3

# Install clean os-core and libstdcpp bundle in target directory
RUN mkdir /install_root \
    && swupd os-install  \
    ${CLEAR_LINUX_VERSION} \
    --path /install_root --statedir /swupd-state \
    --bundles=os-core,libstdcpp --no-boot-update \
    && rm -rf /install_root/var/lib/swupd/*

# Minimal result image
FROM scratch as final
COPY --from=builder /install_root /

# OPAE
# nlb0 and nlb3 examples
COPY --from=builder /usr/src/opae/opae-sdk-*/build/bin/nlb* /usr/bin/
# libxfpga.so, libopae-c*.so*
COPY --from=builder /usr/src/opae/opae-sdk-*/build/lib/lib*.so* /usr/lib64/

COPY test_fpga.sh /usr/bin/
