FROM ubuntu:20.04 AS builder

WORKDIR /dlb-build

# Install build dependencies
RUN apt-get update && apt-get install -y wget xz-utils make gcc

# Download and unpack DLB tarball
ARG DLB_TARBALL="dlb_linux_src_release_8.3.0.txz"
ARG DLB_TARBALL_SHA256="ce2141f055e6ecca9a51ead30b0cd24a89af74afef0daa0440ae48b3f3cbc27d"

RUN wget https://downloadmirror.intel.com/776610/$DLB_TARBALL \
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
