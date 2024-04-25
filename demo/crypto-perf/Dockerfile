FROM debian:stable-slim as builder

ARG DIR=/dpdk-build
WORKDIR $DIR

RUN echo "deb-src http://deb.debian.org/debian unstable main" >> \
    /etc/apt/sources.list.d/deb-src.list
RUN apt-get update && apt-get install -y --no-install-recommends wget build-essential meson ninja-build python3-pyelftools libnuma-dev python3-pip pkg-config dpkg-dev libipsec-mb-dev

# Download & unpack DPDK tarball
ARG DPDK_TARBALL=dpdk-23.11.tar.xz
ARG DPDK_TARBALL_SHA256="64fa58fdfc9e9510e8e414e3bedd165bd3d4ca465a231b280323f83cd53fd865"

RUN wget -q https://git.dpdk.org/dpdk/snapshot/$DPDK_TARBALL \
    && echo "$DPDK_TARBALL_SHA256 $DPDK_TARBALL" | sha256sum -c - \
    && tar -xf $DPDK_TARBALL && rm $DPDK_TARBALL

ARG SOVERSION=24
RUN cd dpdk-* && meson setup \
    -Dplatform=generic \
    -Dcpu_instruction_set=westmere \
    "-Denable_drivers=common/qat,compress/qat,crypto/qat" \
    "-Denable_apps=test-crypto-perf,test-compress-perf" \
    --prefix $(pwd)/installdir \
     builddir
RUN cd dpdk-* && ninja -C builddir install
RUN cd dpdk-* && \
    install -D builddir/app/dpdk-test-crypto-perf /install_root/usr/bin/dpdk-test-crypto-perf && \
    install -D builddir/app/dpdk-test-compress-perf /install_root/usr/bin/dpdk-test-compress-perf && \
    install -d /install_root/usr/lib/x86_64-linux-gnu/ && \
    for r in log bus_pci eal kvargs hash security telemetry pci mbuf mempool ring net rcu ipsec cryptodev compressdev common_qat; do \
       install installdir/lib/x86_64-linux-gnu/librte_${r}.so.${SOVERSION} /install_root/usr/lib/x86_64-linux-gnu/; \
    done

RUN mkdir -p /install_root/licenses/dpdk && \
    cp dpdk-*/license/bsd-3-clause.txt /install_root/licenses/dpdk && \
    cd /install_root/licenses/dpdk && \
    apt-get source --download-only -y libatomic1 libnuma1

ARG TOYBOX_VERSION="0.8.11"
ARG TOYBOX_SHA256="83a3a88cbe1fa30f099c2f58295baef4637aaf988085aaea56e03aa29168175d"

ARG ROOT=/install_root

RUN apt-get update && apt-get --no-install-recommends -y install musl musl-tools musl-dev curl
COPY toybox-config-static toybox-config-static
RUN curl -SL https://github.com/landley/toybox/archive/refs/tags/$TOYBOX_VERSION.tar.gz -o toybox.tar.gz \
    && echo "$TOYBOX_SHA256 toybox.tar.gz" | sha256sum -c - \
    && tar -xzf toybox.tar.gz \
    && rm toybox.tar.gz \
    && cd toybox-$TOYBOX_VERSION \
    && KCONFIG_CONFIG=../toybox-config-static LDFLAGS="--static" CC=musl-gcc PREFIX=$ROOT/bin V=2 make toybox install_flat \
    && install -D LICENSE $ROOT/licenses/toybox \
    && cp -r /usr/share/doc/musl $ROOT/licenses/

ARG LIBS="libnuma libatomic libtinfo libIPSec_MB"
ARG LICENSES="libnuma1 libatomic1 libtinfo6 libipsec-mb1 gcc-12-base"
RUN mkdir /tmp/libs && for l in ${LIBS}; do cp "/lib/x86_64-linux-gnu/${l}.so"* /tmp/libs/; done
RUN mkdir /tmp/licenses && for l in ${LICENSES}; do cp -r "/usr/share/doc/${l}" /tmp/licenses/; done

FROM gcr.io/distroless/cc

COPY --from=builder /tmp/libs/ /lib/x86_64-linux-gnu/
COPY --from=builder /tmp/licenses/ /usr/share/doc/

COPY --from=builder /install_root /
COPY run-dpdk-test /usr/bin/

ENTRYPOINT /usr/bin/run-dpdk-test
