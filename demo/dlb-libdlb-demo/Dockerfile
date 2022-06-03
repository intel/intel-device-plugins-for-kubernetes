FROM ubuntu:20.04 AS builder

WORKDIR /dlb-build

# Install build dependencies
RUN apt-get update && apt-get install -y wget xz-utils make gcc

# Download and unpack DLB Driver tarball
ARG DLB_DRIVER_RELEASE="dlb_linux_src_release7.6.0_2022_03_30.txz"
ARG DLB_DRIVER_SHA256="b74c1bb2863fb6374bf80b9268b5978ab7b9d4eabb2d47ea427a5460aa3ae5fe"

RUN wget https://downloadmirror.intel.com/727424/$DLB_DRIVER_RELEASE \
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
