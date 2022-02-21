FROM ubuntu:20.04 as builder

ARG QAT_DRIVER_RELEASE="QAT.L.4.15.0-00011"
ARG QAT_DRIVER_SHA256="036a87f8c337a3d7a8f6e44e39d9767bb88b3b7866ee00886237d454a628aa56"
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
    wget \
    git \
    yasm \
    autoconf \
    cmake \
    libtool && \
    git clone -b $QAT_ENGINE_VERSION https://github.com/intel/QAT_Engine && \
    git clone -b $IPP_CRYPTO_VERSION https://github.com/intel/ipp-crypto && \
    git clone -b $IPSEC_MB_VERSION https://github.com/intel/intel-ipsec-mb && \
    wget https://downloadmirror.intel.com/649693/$QAT_DRIVER_RELEASE.tar.gz && \
    echo "$QAT_DRIVER_SHA256 $QAT_DRIVER_RELEASE.tar.gz" | sha256sum -c - && \
    tar xf *.tar.gz

RUN sed -i -e 's/cmn_ko$//' -e 's/lac_kernel$//' quickassist/Makefile && \
    KERNEL_SOURCE_ROOT=/tmp ./configure && \
    make quickassist-all adf-ctl-all && \
    install -m 755 build/libqat_s.so /usr/lib/ && \
    install -m 755 build/libusdm_drv_s.so /usr/lib/ && \
    install -m 755 build/adf_ctl /usr/bin/ && \
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
    --with-qat_hw_dir=/ \
    --with-qat_hw_install_dir=/usr/lib \
    --with-qat_engine_id=qat-hw && \
    make && make install && \
    mv /usr/lib/x86_64-linux-gnu/engines-1.1/qatengine.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat-hw.so

FROM ubuntu:20.04

COPY --from=builder /usr/lib/libqat_s.so /usr/lib/
COPY --from=builder /usr/lib/libusdm_drv_s.so /usr/lib/
COPY --from=builder /usr/lib64/libIPSec_MB.so.1 /usr/lib/x86_64-linux-gnu/
COPY --from=builder /usr/local/lib/libcrypto_mb.so.11.3 /usr/lib/x86_64-linux-gnu/
COPY --from=builder /usr/bin/adf_ctl /usr/bin
COPY --from=builder /usr/lib/x86_64-linux-gnu/engines-1.1/qat-sw.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat-sw.so
COPY --from=builder /usr/lib/x86_64-linux-gnu/engines-1.1/qat-hw.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat-hw.so
COPY --from=builder /LICENSE.GPL /usr/share/package-licenses/libqat/LICENSE.GPL
COPY --from=builder /QAT_Engine/LICENSE /usr/share/package-licenses/QAT_Engine/LICENSE
COPY --from=builder /ipp-crypto/LICENSE /usr/share/package-licenses/ipp-crypto/LICENSE
COPY --from=builder /intel-ipsec-mb/LICENSE /usr/share/package-licenses/intel-ipsec-mb/LICENSE
RUN ldconfig && apt update && env DEBIAN_FRONTEND=noninteractive apt install -y openssl
