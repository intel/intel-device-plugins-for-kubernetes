FROM ubuntu:26.04 AS builder

ARG QAT_ENGINE_VERSION="v2.3.0"
# Must match the qatlib version of the qatlib-examples package installed below
ARG QATLIB_VERSION="26.02.0"

RUN apt update && \
    env DEBIAN_FRONTEND=noninteractive apt install --no-install-recommends -y \
    git \
    ca-certificates \
    autoconf \
    automake \
    g++ \
    pkg-config \
    libssl-dev \
    libtool \
    libqat-dev \
    make \
    libusdm-dev

RUN git clone --depth 1 -b $QAT_ENGINE_VERSION https://github.com/intel/QAT_Engine

# OpenSSL in Ubuntu 26.04 is built without ENGINE support, so QAT_Engine is
# built as an OpenSSL provider (qatprovider.so). The sed makes the provider
# fail to load instead of silently falling back to OpenSSL SW when no QAT HW
# device is available.
RUN cd /QAT_Engine && \
    sed -i '/WARN("QAT_HW device not available & QAT_SW not enabled. Using OpenSSL_SW!\\n");/ s/$/ return 0;/' e_qat.c && \
    ./autogen.sh && \
    ./configure \
    --disable-qat_sw && \
    make && make test && make install

# qatlib-examples in Ubuntu 26.04 does not ship the compression corpus files
# (/usr/share/qat/) that cpa_sample_code needs for the compression tests.
RUN git clone --depth 1 -b $QATLIB_VERSION https://github.com/intel/qatlib && \
    install -D -m 0644 -t /usr/share/qat \
    qatlib/quickassist/lookaside/access_layer/src/sample_code/performance/compression/calgary \
    qatlib/quickassist/lookaside/access_layer/src/sample_code/performance/compression/calgary32 \
    qatlib/quickassist/lookaside/access_layer/src/sample_code/performance/compression/canterbury

FROM ubuntu:26.04

COPY --from=builder /QAT_Engine/testapp /usr/bin/
COPY --from=builder /usr/lib/x86_64-linux-gnu/ossl-modules/qatprovider.so /usr/lib/x86_64-linux-gnu/ossl-modules/qatprovider.so
COPY --from=builder /QAT_Engine/LICENSE /usr/share/package-licenses/QAT_Engine/LICENSE
COPY --from=builder /usr/share/qat /usr/share/qat
RUN apt update && env DEBIAN_FRONTEND=noninteractive apt install --no-install-recommends -y openssl libqat4 libusdm0 qatlib-examples
