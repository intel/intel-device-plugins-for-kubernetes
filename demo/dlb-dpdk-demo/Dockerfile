FROM ubuntu:20.04 as builder

ARG DIR=/dpdk-build
WORKDIR $DIR

RUN apt-get update && apt-get install -y wget build-essential meson python3-pyelftools libnuma-dev python3-pip
RUN pip install ninja

# Download & unpack DLB tarball
ARG DLB_TARBALL="dlblinuxsrcrelease7.4.020211006.txz"
ARG DLB_TARBALL_SHA256="715c34314d77dce9fe0cd61d5c0f269016971e8d9aa680bc0e4a32d4284aae09"

RUN wget https://01.org/sites/default/files/downloads/$DLB_TARBALL \
    && echo "$DLB_TARBALL_SHA256 $DLB_TARBALL" | sha256sum -c - \
    && tar -Jxf $DLB_TARBALL --no-same-owner && rm $DLB_TARBALL

# Download & unpack DPDK tarball
ARG DPDK_TARBALL=dpdk-20.11.3.tar.xz
ARG DPDK_TARBALL_SHA256="898680458a4010f421fab760aef47369b74d2954e3560841df3cd7b98076b841"

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
