FROM debian:sid as builder

ENV QAT_DRIVER_RELEASE="qat1.7.l.4.3.0-00033"
ENV QAT_ENGINE_VERSION="v0.5.41"

RUN apt-get update && \
    apt-get install -y git build-essential wget libssl-dev openssl libudev-dev pkg-config autoconf autogen libtool gawk && \
    git clone https://github.com/intel/QAT_Engine && \
    wget https://01.org/sites/default/files/downloads/intelr-quickassist-technology/$QAT_DRIVER_RELEASE.tar.gz && \
    tar zxf $QAT_DRIVER_RELEASE.tar.gz


RUN sed -i -e 's/cmn_ko$//' -e 's/lac_kernel$//' quickassist/Makefile && \
    KERNEL_SOURCE_ROOT=/tmp ./configure && \
    make quickassist-all adf-ctl-all && \
    install -m 755 build/libqat_s.so /usr/lib/ && \
    install -m 755 build/libusdm_drv_s.so /usr/lib/ && \
    install -m 755 build/adf_ctl /usr/bin/ && \
    cd QAT_Engine && git checkout $QAT_ENGINE_VERSION && \
    ./autogen.sh && \
    ./configure \
    --with-qat_dir=/ \
    --with-openssl_dir=/usr \
    --with-openssl_install_dir=/usr/lib/x86_64-linux-gnu \
    --enable-upstream_driver \
    --enable-usdm \
    --with-qat_install_dir=/usr/lib \
    --enable-qat_skip_err_files_build \
    --enable-openssl_install_build_arch_path && \
    make && make install

FROM debian:sid-slim

RUN apt-get update && apt-get install -y openssl

COPY --from=builder /usr/lib/libqat_s.so /usr/lib/
COPY --from=builder /usr/lib/libusdm_drv_s.so /usr/lib/
COPY --from=builder /usr/bin/adf_ctl /usr/bin
COPY --from=builder /usr/lib/x86_64-linux-gnu/engines-1.1/qat.so /usr/lib/x86_64-linux-gnu/engines-1.1/qat.so
