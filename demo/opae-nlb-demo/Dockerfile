# Fedora is a recommended distro for OPAE
# https://opae.github.io/latest
ARG FEDORA_RELEASE="32"
FROM fedora:${FEDORA_RELEASE} as builder

# Install build dependencies
RUN dnf install -y git-core curl cmake gcc gcc-c++ make spdlog-devel kernel-headers libedit-devel libuuid-devel json-c-devel cli11-devel python-devel

# Download and unpack OPAE tarball
ARG OPAE_RELEASE=2.0.2-1
ARG OPAE_SHA256=2cc4d55d6b41eb0dee6927b0984329c0eee798b2e183dc434479757ae603b5e1

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
    make -j xfpga nlb0 nlb3

# Copy required nlb* utils and their dependencies to the final image
FROM debian:unstable-slim
COPY --from=builder /usr/src/opae/opae-sdk-*/build/bin/nlb* /usr/local/bin/
COPY --from=builder /usr/src/opae/opae-sdk-*/build/lib /usr/local/lib/
RUN rm -rf /usr/local/lib/python3
RUN ldconfig

RUN apt-get update && apt-get install -y libjson-c4

COPY test_fpga.sh /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/test_fpga.sh"]
