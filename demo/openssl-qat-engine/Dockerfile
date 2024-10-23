FROM ubuntu:24.04 AS builder

ARG QAT_ENGINE_VERSION="v1.6.2"

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

RUN cd /QAT_Engine && \
    sed -i '/fprintf(stderr, "QAT_HW device not available & QAT_SW not enabled. Using OpenSSL_SW!\\n");/ s/$/ return ret;/' e_qat.c && \
    ./autogen.sh && \
    ./configure \
    --disable-qat_sw \
    --with-qat_engine_id=qathwtest && \
    make && make test && make install

FROM ubuntu:24.04

COPY --from=builder /QAT_Engine/testapp /usr/bin/
COPY --from=builder /usr/lib/x86_64-linux-gnu/engines-3/qatengine.so /usr/lib/x86_64-linux-gnu/engines-3/qathwtest.so
COPY --from=builder /QAT_Engine/LICENSE /usr/share/package-licenses/QAT_Engine/LICENSE
RUN apt update && env DEBIAN_FRONTEND=noninteractive apt install --no-install-recommends -y qatengine qatlib-examples
