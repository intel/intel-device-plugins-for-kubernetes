FROM ubuntu:20.04 AS builder

WORKDIR /dlb-build

# Install build dependencies
RUN apt-get update && apt-get install -y wget xz-utils make gcc

# Download and unpack DLB tarball
ARG DLB_TARBALL="dlb_linux_src_release8.0.0.txz"
ARG DLB_TARBALL_SHA256="075533229bb2bd2f945ec8089a707205f3f8e8d87a8030e9208603d997236171"

RUN wget https://downloadmirror.intel.com/763709/$DLB_TARBALL \
    && echo "$DLB_TARBALL_SHA256 $DLB_TARBALL" | sha256sum -c - \
    && tar -xvf *.txz --no-same-owner

# Build libdlb
RUN cd dlb/libdlb && make

FROM ubuntu:20.04
COPY --from=builder /dlb-build/dlb/libdlb/libdlb.so /usr/local/lib
RUN ldconfig

COPY --from=builder /dlb-build/dlb/libdlb/examples/*traffic /usr/local/bin/
COPY test.sh /usr/bin/

ENTRYPOINT /usr/bin/test.sh
