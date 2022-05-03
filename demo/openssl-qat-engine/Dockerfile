FROM ubuntu:20.04 as builder

ARG QATLIB_VERSION="21.11.0"
ARG QAT_ENGINE_VERSION="v0.6.11"
ARG IPSEC_MB_VERSION="v1.1"
ARG IPP_CRYPTO_VERSION="ippcp_2021.5"

RUN apt update && \
    env DEBIAN_FRONTEND=noninteractive apt install -y \
    libudev-dev \
    make \
    gcc \
    g++ \
    nasm \
    pkg-config \
    libssl-dev \
    zlib1g-dev \
    wget \
    git \
    yasm \
    autoconf \
    cmake \
    libtool && \
    git clone -b $QAT_ENGINE_VERSION https://github.com/intel/QAT_Engine && \
    git clone -b $IPP_CRYPTO_VERSION https://github.com/intel/ipp-crypto && \
    git clone -b $IPSEC_MB_VERSION https://github.com/intel/intel-ipsec-mb && \
    git clone -b $QATLIB_VERSION https://github.com/intel/qatlib

RUN cd /qatlib && \
    sed -i -e '79,87d' configure.ac && \
    ./autogen.sh && \
    ./configure --prefix=/usr && \
    make -j && \
    make install samples-install && \
    cd /ipp-crypto/sources/ippcp/crypto_mb && \
    cmake . -B"../build" \
    -DOPENSSL_INCLUDE_DIR=/usr/include/openssl \
    -DOPENSSL_LIBRARIES=/usr/lib64 \
    -DOPENSSL_ROOT_DIR=/usr/bin/openssl && \
    cd ../build && \
    make crypto_mb && make install && \
    cd /intel-ipsec-mb && \
    make && make install LIB_INSTALL_DIR=/usr/lib64

# Build QAT Engine twice: ISA optimized qat-sw and QAT HW
# optimized qat-hw.
#
# NB: The engine build needs 'make clean' between the builds but
# that removes the installed engine too. Therefore, we need to
# take a backup before 'make clean' and restore it afterwards.
# See: https://github.com/intel/QAT_Engine/issues/172
RUN cd /QAT_Engine && \
    ./autogen.sh && \
    ./configure \
    --enable-qat_sw \
    --with-qat_sw_install_dir=/usr/local \
    --with-qat_engine_id=qat-sw && \
    make && make install && \
    mv /usr/lib/x86_64-linux-gnu/engines-1.1/qatengine.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat-sw.so.tmp && \
    make clean && \
    mv /usr/lib/x86_64-linux-gnu/engines-1.1/qat-sw.so.tmp /usr/lib/x86_64-linux-gnu/engines-1.1/qat-sw.so && \
    ./configure \
    --with-qat_engine_id=qat-hw && \
    make && make install && \
    mv /usr/lib/x86_64-linux-gnu/engines-1.1/qatengine.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat-hw.so

FROM ubuntu:20.04

COPY --from=builder /usr/bin/*_sample* /usr/bin/
COPY --from=builder /usr/lib/libqat.so.2.0.0 /usr/lib/
COPY --from=builder /usr/lib/libusdm.so.0.0.1 /usr/lib/
COPY --from=builder /usr/lib64/libIPSec_MB.so.1 /usr/lib/x86_64-linux-gnu/
COPY --from=builder /usr/local/lib/libcrypto_mb.so.11.3 /usr/lib/x86_64-linux-gnu/
COPY --from=builder /usr/lib/x86_64-linux-gnu/engines-1.1/qat-sw.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat-sw.so
COPY --from=builder /usr/lib/x86_64-linux-gnu/engines-1.1/qat-hw.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat-hw.so
COPY --from=builder /qatlib/LICENSE /usr/share/package-licenses/qatlib/LICENSE
COPY --from=builder /QAT_Engine/LICENSE /usr/share/package-licenses/QAT_Engine/LICENSE
COPY --from=builder /ipp-crypto/LICENSE /usr/share/package-licenses/ipp-crypto/LICENSE
COPY --from=builder /intel-ipsec-mb/LICENSE /usr/share/package-licenses/intel-ipsec-mb/LICENSE
RUN ldconfig && apt update && env DEBIAN_FRONTEND=noninteractive apt install -y openssl haproxy
