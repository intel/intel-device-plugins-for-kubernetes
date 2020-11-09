ARG FINAL_BASE_IMAGE=clearlinux:base

FROM clearlinux:base as builder

ARG QAT_DRIVER_RELEASE="qat1.7.l.4.11.0-00001"
ARG QAT_ENGINE_VERSION="v0.6.1"
ARG IPSEC_MB_VERSION="v0.55"
ARG IPP_CRYPTO_VERSION="ippcp_2020u3"

RUN swupd bundle-add --skip-diskspace-check devpkg-systemd devpkg-openssl c-basic wget git && \
    git clone -b $QAT_ENGINE_VERSION https://github.com/intel/QAT_Engine && \
    git clone -b $IPP_CRYPTO_VERSION https://github.com/intel/ipp-crypto && \
    git clone -b $IPSEC_MB_VERSION https://github.com/intel/intel-ipsec-mb && \
    wget https://01.org/sites/default/files/downloads/$QAT_DRIVER_RELEASE.tar.gz && \
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
    --enable-ipsec_offload \
    --enable-multibuff_offload \
    --with-multibuff_install_dir=/usr/local \
    --with-qat_engine_id=qat-sw && \
    make && make install && \
    mv /usr/lib64/engines-1.1/qat-sw.so /usr/lib64/engines-1.1/qat-sw.so.tmp && \
    make clean && \
    mv /usr/lib64/engines-1.1/qat-sw.so.tmp /usr/lib64/engines-1.1/qat-sw.so && \
    ./configure \
    --with-qat_dir=/ \
    --disable-ipsec_offload \
    --disable-multibuff_offload \
    --with-qat_install_dir=/usr/lib \
    --with-qat_engine_id=qat-hw && \
    make && make install

FROM $FINAL_BASE_IMAGE

COPY --from=builder /usr/lib/libqat_s.so /usr/lib/
COPY --from=builder /usr/lib/libusdm_drv_s.so /usr/lib/
COPY --from=builder /usr/lib64/libIPSec_MB.so.0 /usr/lib64/
COPY --from=builder /usr/local/lib/libcrypto_mb.so /usr/lib64/
COPY --from=builder /usr/bin/adf_ctl /usr/bin
COPY --from=builder /usr/lib64/engines-1.1/qat-sw.so /usr/lib64/engines-1.1/qat-sw.so
COPY --from=builder /usr/lib64/engines-1.1/qat-hw.so /usr/lib64/engines-1.1/qat-hw.so
COPY --from=builder /LICENSE.GPL /usr/share/package-licenses/libqat/LICENSE.GPL
COPY --from=builder /QAT_Engine/LICENSE /usr/share/package-licenses/QAT_Engine/LICENSE
COPY --from=builder /ipp-crypto/LICENSE /usr/share/package-licenses/ipp-crypto/LICENSE
COPY --from=builder /intel-ipsec-mb/LICENSE /usr/share/package-licenses/intel-ipsec-mb/LICENSE
