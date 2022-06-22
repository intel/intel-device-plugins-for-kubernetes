FROM ubuntu:20.04 AS builder

WORKDIR /dlb-build

# Install build dependencies
RUN apt-get update && apt-get install -y wget xz-utils make gcc

# Download and unpack DLB tarball
ARG DLB_DRIVER_RELEASE="dlb_linux_src_release_7.7.0_2022_06_17.txz"
ARG DLB_DRIVER_SHA256="b596bdcc1ace67a1e1f994d98be33561f5fae26b92bfbd26027a793d4cb54c8d"

RUN wget https://downloadmirror.intel.com/734482/$DLB_TARBALL \
    && echo "$DLB_DRIVER_SHA256 $DLB_DRIVER_RELEASE" | sha256sum -c - \
    && tar -xvf *.txz --no-same-owner

# Build libdlb
RUN cd dlb/libdlb && make

FROM ubuntu:20.04
COPY --from=builder /dlb-build/dlb/libdlb/libdlb.so /usr/local/lib
RUN ldconfig

COPY --from=builder /dlb-build/dlb/libdlb/examples/*traffic /usr/local/bin/
COPY test.sh /usr/bin/

ENTRYPOINT /usr/bin/test.sh
