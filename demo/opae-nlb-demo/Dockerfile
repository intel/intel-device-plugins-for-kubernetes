FROM debian:stable-slim AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y curl python3-dev git gcc g++ make cmake uuid-dev libjson-c5 libjson-c-dev libedit-dev libudev-dev

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

ARG TOYBOX_VERSION="0.8.11"
ARG TOYBOX_SHA256="83a3a88cbe1fa30f099c2f58295baef4637aaf988085aaea56e03aa29168175d"

ARG ROOT=/install_root

RUN apt-get update && apt-get --no-install-recommends -y install musl musl-tools musl-dev curl
COPY toybox-config-static toybox-config-static
RUN curl -SL https://github.com/landley/toybox/archive/refs/tags/$TOYBOX_VERSION.tar.gz -o toybox.tar.gz \
    && echo "$TOYBOX_SHA256 toybox.tar.gz" | sha256sum -c - \
    && tar -xzf toybox.tar.gz \
    && rm toybox.tar.gz \
    && cd toybox-$TOYBOX_VERSION \
    && KCONFIG_CONFIG=../toybox-config-static LDFLAGS="--static" CC=musl-gcc PREFIX=$ROOT/bin V=2 make toybox install_flat \
    && install -D LICENSE $ROOT/licenses/toybox \
    && cp -r /usr/share/doc/musl $ROOT/licenses/

ARG LIBS="libjson-c libuuid"
ARG LICENSES="libjson-c5 libuuid1 gcc-12-base"
RUN mkdir /tmp/libs && for l in ${LIBS}; do cp "/lib/x86_64-linux-gnu/${l}"* /tmp/libs/; done
RUN mkdir /tmp/licenses && for l in ${LICENSES}; do cp -r "/usr/share/doc/${l}" /tmp/licenses/; done

FROM gcr.io/distroless/cc

COPY --from=builder /tmp/libs/* /lib/x86_64-linux-gnu/
COPY --from=builder /tmp/licenses/ /usr/share/doc/
COPY --from=builder /install_root /

# Copy required nlb* utils and their dependencies to the final image
COPY --from=builder /usr/src/opae/opae-sdk-*/build/bin/nlb* /usr/local/bin/
COPY --from=builder /usr/src/opae/opae-sdk-*/build/lib /lib/x86_64-linux-gnu/
COPY --from=builder /usr/src/opae/opae-sdk-*/COPYING /usr/local/share/package-licenses/opae.COPYING

COPY test_fpga.sh /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/test_fpga.sh"]
