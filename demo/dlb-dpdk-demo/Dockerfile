FROM ubuntu:20.04 as builder

ARG DIR=/dpdk-build
WORKDIR $DIR

RUN apt-get update && apt-get install -y wget build-essential meson python3-pyelftools libnuma-dev python3-pip
RUN pip install ninja

# Download & unpack DLB tarball
ARG DLB_TARBALL="dlb_linux_src_release7.6.0_2022_03_30.txz"
ARG DLB_TARBALL_SHA256="b74c1bb2863fb6374bf80b9268b5978ab7b9d4eabb2d47ea427a5460aa3ae5fe"

RUN wget https://downloadmirror.intel.com/727424/$DLB_TARBALL \
    && echo "$DLB_TARBALL_SHA256 $DLB_TARBALL" | sha256sum -c - \
    && tar -Jxf $DLB_TARBALL --no-same-owner && rm $DLB_TARBALL

# Download & unpack DPDK tarball
ARG DPDK_TARBALL=dpdk-20.11.4.tar.xz
ARG DPDK_TARBALL_SHA256="78028c6a9f4d247b5215ca156b6dbeb03f68a99ca00109c347615a46c1856d6a"

RUN wget -q https://fast.dpdk.org/rel/$DPDK_TARBALL \
    && echo "$DPDK_TARBALL_SHA256 $DPDK_TARBALL" | sha256sum -c - \
    && tar -xf $DPDK_TARBALL && rm $DPDK_TARBALL

RUN cd dpdk-* && patch -Np1 < $(echo ../dlb/dpdk/dpdk_dlb_*.patch) && meson setup --prefix $(pwd)/installdir builddir
RUN cd dpdk-* && ninja -C builddir install && install -D builddir/app/dpdk-test-eventdev /install_root/usr/bin/dpdk-test-eventdev

FROM ubuntu:20.04
RUN apt-get update && apt-get install -y libnuma1
COPY --from=builder /install_root /
COPY test.sh /usr/bin/

ENTRYPOINT /usr/bin/test.sh
