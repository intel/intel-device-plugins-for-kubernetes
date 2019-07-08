FROM clearlinux:base as builder

# Move to latest Clear Linux release
# ARG swupd_args
# RUN swupd update --no-boot-update $swupd_args

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
RUN source /usr/lib/os-release \
    && mkdir /install_root \
    && swupd os-install -V ${VERSION_ID} \
    --path /install_root --statedir /swupd-state \
    --bundles=os-core,libstdcpp --no-scripts \
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
