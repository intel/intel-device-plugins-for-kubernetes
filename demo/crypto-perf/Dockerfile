FROM debian:sid-slim as builder

ARG DIR=/dpdk-build
WORKDIR $DIR

RUN echo "deb-src http://deb.debian.org/debian unstable main" >> \
    /etc/apt/sources.list.d/deb-src.list
RUN apt-get update && apt-get install -y wget build-essential meson python3-pyelftools libnuma-dev python3-pip libssl-dev pkg-config dpkg-dev

# Download & unpack DPDK tarball
ARG DPDK_TARBALL=dpdk-22.11.tar.xz
ARG DPDK_TARBALL_SHA256="8eefcc69afa87dccaf8d730d805ded70fb8b64905295d6396977c1322e59eadb"

ARG SOVERSION=23
RUN wget -q https://git.dpdk.org/dpdk/snapshot/$DPDK_TARBALL \
    && echo "$DPDK_TARBALL_SHA256 $DPDK_TARBALL" | sha256sum -c - \
    && tar -xf $DPDK_TARBALL && rm $DPDK_TARBALL

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
    for r in bus_pci eal kvargs hash security telemetry pci mbuf mempool ring net rcu ipsec cryptodev compressdev common_qat; do \
       install installdir/lib/x86_64-linux-gnu/librte_${r}.so.${SOVERSION} /install_root/usr/lib/x86_64-linux-gnu/; \
    done

RUN mkdir -p /install_root/licenses/dpdk && \
    cp dpdk-*/license/bsd-3-clause.txt /install_root/licenses/dpdk && \
    cd /install_root/licenses/dpdk && \
    apt-get source --download-only -y libatomic1 libnuma1

FROM debian:sid-slim
RUN apt-get update && apt-get install -y libssl3 libnuma1 libatomic1 && ldconfig -v
COPY --from=builder /install_root /
COPY run-dpdk-test /usr/bin/

ENTRYPOINT /usr/bin/run-dpdk-test
