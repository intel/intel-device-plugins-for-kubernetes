FROM ubuntu:20.04 as builder

ARG DIR=/dpdk-build
WORKDIR $DIR

RUN apt-get update && apt-get install -y wget build-essential meson python3-pyelftools libnuma-dev python3-pip

# Download & unpack DLB tarball
ARG DLB_TARBALL="dlb_linux_src_release_7.7.0_2022_06_17.txz"
ARG DLB_TARBALL_SHA256="b596bdcc1ace67a1e1f994d98be33561f5fae26b92bfbd26027a793d4cb54c8d"

RUN wget https://downloadmirror.intel.com/734482/$DLB_TARBALL \
    && echo "$DLB_TARBALL_SHA256 $DLB_TARBALL" | sha256sum -c - \
    && tar -Jxf $DLB_TARBALL --no-same-owner && rm $DLB_TARBALL

# Download & unpack DPDK tarball
ARG DPDK_TARBALL=dpdk-21.11.tar.xz
ARG DPDK_TARBALL_SHA256="3246e3ed68ee2b369a5d8be2c06cf108a669e157f4d41c5bcbbb216bf5abd3a1"

RUN wget -q https://fast.dpdk.org/rel/$DPDK_TARBALL \
    && echo "$DPDK_TARBALL_SHA256 $DPDK_TARBALL" | sha256sum -c - \
    && tar -xf $DPDK_TARBALL && rm $DPDK_TARBALL

RUN cd dpdk-* && patch -Np1 < $(echo ../dlb/dpdk/dpdk_dlb_*.patch) && sed -i 's/270b,2710,2714/270b,2710,2711,2714/g' ./usertools/dpdk-devbind.py && meson setup --prefix $(pwd)/installdir builddir
RUN cd dpdk-* && ninja -C builddir install && install -D builddir/app/dpdk-test-eventdev /install_root/usr/bin/dpdk-test-eventdev

FROM ubuntu:20.04
RUN apt-get update && apt-get install -y libnuma1 libatomic1
COPY --from=builder /install_root /
COPY test.sh /usr/bin/

ENTRYPOINT /usr/bin/test.sh
