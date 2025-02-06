FROM debian:sid-slim AS builder

ARG DIR=/dpdk-build
WORKDIR $DIR

RUN apt-get update && apt-get install -y --no-install-recommends wget build-essential meson ninja-build python3-pyelftools libnuma-dev python3-pip pkg-config dpkg-dev libipsec-mb-dev

# Download & unpack DPDK tarball
ARG DPDK_TARBALL=dpdk-24.11.1.tar.xz
ARG DPDK_TARBALL_SHA256="bcae7d42c449fc456dfb279feabcbe0599a29bebb2fe2905761e187339d96b8e"

RUN wget -q https://fast.dpdk.org/rel/$DPDK_TARBALL \
    && echo "$DPDK_TARBALL_SHA256 $DPDK_TARBALL" | sha256sum -c - \
    && tar -xf $DPDK_TARBALL && rm $DPDK_TARBALL

RUN cd dpdk-* && meson setup \
    "-Dplatform=generic" \
    "-Denable_drivers=dma/idxd" \
    "-Denable_apps=test" \
    --prefix $(pwd)/installdir \
     builddir
RUN cd dpdk-* && ninja -C builddir install && install -D installdir/bin/dpdk-test /install_root/usr/bin/dpdk-test

FROM debian:sid-slim
RUN apt-get update && apt-get install -y --no-install-recommends libipsec-mb2 libnuma1 libatomic1 && ldconfig -v
COPY --from=builder /install_root/usr/bin/dpdk-test /usr/bin/dpdk-test

ENTRYPOINT ["dpdk-test", "dmadev_autotest"]
