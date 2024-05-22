FROM debian:unstable-slim AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y curl python3-dev git gcc g++ make cmake uuid-dev libjson-c-dev libedit-dev libudev-dev

# Download and unpack OPAE tarball
ARG OPAE_RELEASE=2.2.0-1
ARG OPAE_SHA256=1f80f73a652b456adee87a260c7ec66fe3cbb35184b7d7b531766fb6bb0295d7

SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN mkdir -p /usr/src/opae && \
    cd /usr/src/opae && \
    curl -fsSL https://github.com/OFS/opae-sdk/archive/${OPAE_RELEASE}.tar.gz -o opae.tar.gz && \
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


FROM debian:unstable-slim

RUN apt-get update && apt-get install --no-install-recommends -y libjson-c5

# Copy required nlb* utils and their dependencies to the final image
COPY --from=builder /usr/src/opae/opae-sdk-*/build/bin/nlb* /usr/local/bin/
COPY --from=builder /usr/src/opae/opae-sdk-*/build/lib /usr/local/lib/
COPY --from=builder /usr/src/opae/opae-sdk-*/COPYING /usr/local/share/package-licenses/opae.COPYING
RUN rm -rf /usr/local/lib/python3 && ldconfig

COPY test_fpga.sh /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/test_fpga.sh"]
