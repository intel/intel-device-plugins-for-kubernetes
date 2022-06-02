FROM fedora:35 AS builder

RUN dnf install -y \
    wget autoconf automake libtool m4 diffutils file make dnf-utils patch \
    libuuid-devel json-c-devel

ARG ACCEL_CONFIG_VERSION=v3.4.6.3

RUN wget -O- https://github.com/intel/idxd-config/archive/accel-config-$ACCEL_CONFIG_VERSION.tar.gz | tar -zx

ADD idxd-reset.patch /
ADD test_runner_disable_shared_queues.patch /

RUN cd idxd-config-accel-config-$ACCEL_CONFIG_VERSION && \
    patch -p1 < ../idxd-reset.patch && \
    patch -p1 < ../test_runner_disable_shared_queues.patch && \
    mkdir m4 && \
    ./autogen.sh && \
    ./configure CFLAGS='-g -O2' --prefix=/usr --sysconfdir=/etc --libdir=/usr/lib64 --enable-test=yes --disable-docs && \
    make && \
    make install

FROM fedora:35

RUN dnf install -y libuuid json-c

COPY --from=builder /lib64/libaccel-config.so.1 /lib64/
COPY --from=builder /lib64/libaccel-config.so.1.0.0 /lib64/
RUN ldconfig

COPY --from=builder /usr/bin/accel-config /usr/bin/
COPY --from=builder /usr/share/accel-config/test /test
COPY --from=builder /idxd-reset.patch /usr/local/share/package-sources/

ENTRYPOINT cd /test && /bin/bash -e ./dsa_user_test_runner.sh
