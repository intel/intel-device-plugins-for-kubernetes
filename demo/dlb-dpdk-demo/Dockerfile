FROM ubuntu:20.04 as builder

ARG DIR=/dpdk-build
WORKDIR $DIR

RUN apt-get update && apt-get install -y --no-install-recommends wget build-essential meson ninja-build python3-pyelftools libnuma-dev python3-pip

# Download & unpack DLB tarball
ARG DLB_TARBALL="dlb_linux_src_release_8.5.2.txz"
ARG DLB_TARBALL_SHA256="99b666e7ce948e20a91e8c2507ec972e10b35fcb1f2061247343dabfdf71d9dd"

RUN wget https://downloadmirror.intel.com/791459/$DLB_TARBALL \
    && echo "$DLB_TARBALL_SHA256 $DLB_TARBALL" | sha256sum -c - \
    && tar -Jxf $DLB_TARBALL --no-same-owner && rm $DLB_TARBALL

# Download & unpack DPDK tarball
ARG DPDK_TARBALL=dpdk-22.11.2.tar.xz
ARG DPDK_TARBALL_SHA256="af64bdda15087ff8d429894b9ea6cbbbb6ee7a932bdb344f82b0dc366379a2d4"

RUN wget -q https://fast.dpdk.org/rel/$DPDK_TARBALL \
    && echo "$DPDK_TARBALL_SHA256 $DPDK_TARBALL" | sha256sum -c - \
    && tar -xf $DPDK_TARBALL && rm $DPDK_TARBALL

RUN cd dpdk-* && patch -Np1 < $(echo ../dlb/dpdk/dpdk_dlb_*.patch) && sed -i 's/270b,2710,2714/270b,2710,2711,2714/g' ./usertools/dpdk-devbind.py && meson setup --prefix $(pwd)/installdir builddir
RUN cd dpdk-* && ninja -C builddir install && install -D builddir/app/dpdk-test-eventdev /install_root/usr/bin/dpdk-test-eventdev

FROM ubuntu:20.04
RUN apt-get update && apt-get install -y --no-install-recommends libnuma1 libatomic1
COPY --from=builder /install_root /
COPY test.sh /usr/bin/

ENTRYPOINT /usr/bin/test.sh
