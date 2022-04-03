FROM fedora:32 AS builder

RUN dnf install -y wget autoconf automake libtool m4 diffutils file make dnf-utils
RUN dnf install -y libuuid-devel json-c-devel kmod-devel libudev-devel

ARG ACCEL_CONFIG_VERSION=v2.8

RUN wget -O- https://github.com/intel/idxd-config/archive/accel-config-$ACCEL_CONFIG_VERSION.tar.gz | tar -zx

RUN cd idxd-config-accel-config-$ACCEL_CONFIG_VERSION && \
    mkdir m4 && \
    ./autogen.sh && \
    ./configure CFLAGS='-g -O2' --prefix=/usr --sysconfdir=/etc --libdir=/usr/lib64 --enable-test=yes --disable-docs && \
    make && \
    make check && \
    make install

FROM fedora:32

RUN dnf install -y libuuid json-c kmod udev

COPY --from=builder /lib64/libaccel-config.so.1 /lib64/
COPY --from=builder /lib64/libaccel-config.so.1.0.0 /lib64/
RUN ldconfig

COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /usr/share/accel-config/test /test

ENTRYPOINT cd /test && sed '/_cleanup$/d;/start_dsa$/d;/enable_wqs$/d;/stop_dsa$/d;/disable_wqs$/d' dsa_user_test_runner.sh | sh
