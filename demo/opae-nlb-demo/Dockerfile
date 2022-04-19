# Fedora is a recommended distro for OPAE
# https://opae.github.io/latest
ARG FEDORA_RELEASE="34"
FROM fedora:${FEDORA_RELEASE} as builder

# Install build dependencies
RUN dnf install -y git-core curl cmake gcc gcc-c++ make kernel-headers libedit-devel libuuid-devel json-c-devel python-devel

# Download and unpack OPAE tarball
ARG OPAE_RELEASE=2.1.0-1
ARG OPAE_SHA256=0c9f52ff1279ca5e34e64828ddf3a6c001fb2969a6349a966cd9515c1048ea01

RUN mkdir -p /usr/src/opae && \
    cd /usr/src/opae && \
    curl -fsSL https://github.com/OPAE/opae-sdk/archive/${OPAE_RELEASE}.tar.gz -o opae.tar.gz && \
    echo "$OPAE_SHA256 opae.tar.gz" | sha256sum -c - && \
    tar -xzf opae.tar.gz && \
    rm -f opae.tar.gz

# Build OPAE
RUN cd /usr/src/opae/opae-sdk-${OPAE_RELEASE} && \
    mkdir build && \
    cd build && \
    CFLAGS="$CFLAGS -Wno-misleading-indentation" \
    cmake .. && \
    make -j nlb0 nlb3

# Copy required nlb* utils and their dependencies to the final image

FROM debian:unstable-slim

COPY --from=builder /usr/src/opae/opae-sdk-*/build/bin/nlb* /usr/local/bin/
COPY --from=builder /usr/src/opae/opae-sdk-*/build/lib /usr/local/lib/
COPY --from=builder /usr/src/opae/opae-sdk-*/COPYING /usr/local/share/package-licenses/opae.COPYING
COPY --from=builder /usr/lib64/libjson-c.so.5.0.0 /usr/local/lib/
COPY --from=builder /usr/share/licenses/json-c /usr/local/share/package-licenses/json-c
RUN rm -rf /usr/local/lib/python3
RUN ldconfig

COPY test_fpga.sh /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/test_fpga.sh"]
