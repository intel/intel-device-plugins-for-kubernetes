ARG FINAL_BASE_IMAGE=clearlinux:base

FROM clearlinux:base as builder

ARG QAT_DRIVER_RELEASE="qat1.7.l.4.10.0-00014"
ARG QAT_ENGINE_VERSION="v0.5.46"
ARG IPSEC_MB_VERSION="v0.54"
ARG IPP_CRYPTO_VERSION="ippcp_2020u2"

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
    install -m 755 bin/vfy_ifma_cp_rsa_mb /usr/bin/ && \
    install -m 755 bin/vfy_ifma_rsa_mb /usr/bin/ && \
    cd /intel-ipsec-mb && \
    make && make install LIB_INSTALL_DIR=/usr/lib64

RUN cd /QAT_Engine && \
    ./autogen.sh && \
    ./configure \
    --enable-ipsec_offload \
    --enable-multibuff_offload \
    --with-openssl_install_dir=/usr/lib64 \
    --with-multibuff_install_dir=/usr/local \
    --enable-openssl_install_build_arch_path && \
    sed -i -e 's:^\(const char \*engine_qat_id = \).*:\1"avx512";:' e_qat.c && \
    make && make install && \
    mv /usr/lib64/engines-1.1/qat.so /usr/lib64/engines-1.1/avx512.so && \
    make clean && git restore e_qat.c && \
    ./configure \
    --with-qat_dir=/ \
    --disable-ipsec_offload \
    --disable-multibuff_offload \
    --with-openssl_install_dir=/usr/lib64 \
    --with-qat_install_dir=/usr/lib \
    --enable-openssl_install_build_arch_path && \
    make && make install

FROM $FINAL_BASE_IMAGE

COPY --from=builder /usr/lib/libqat_s.so /usr/lib/
COPY --from=builder /usr/lib/libusdm_drv_s.so /usr/lib/
COPY --from=builder /usr/lib64/libIPSec_MB.so.0 /usr/lib64/
COPY --from=builder /usr/bin/adf_ctl /usr/bin
COPY --from=builder /usr/bin/vfy_ifma_cp_rsa_mb /usr/bin
COPY --from=builder /usr/bin/vfy_ifma_rsa_mb /usr/bin
COPY --from=builder /usr/lib64/engines-1.1/avx512.so /usr/lib64/engines-1.1/avx512.so
COPY --from=builder /usr/lib64/engines-1.1/qat.so /usr/lib64/engines-1.1/qat.so
COPY --from=builder /LICENSE.GPL /usr/share/package-licenses/libqat/LICENSE.GPL
COPY --from=builder /QAT_Engine/LICENSE /usr/share/package-licenses/QAT_Engine/LICENSE
COPY --from=builder /ipp-crypto/LICENSE /usr/share/package-licenses/ipp-crypto/LICENSE
COPY --from=builder /intel-ipsec-mb/LICENSE /usr/share/package-licenses/intel-ipsec-mb/LICENSE
