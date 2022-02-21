FROM ubuntu:18.04 AS builder

WORKDIR /dlb-build

# Install build dependencies
RUN apt-get update && apt-get install -y wget xz-utils make gcc

# Download and unpack DLB Driver tarball
ARG DLB_DRIVER_RELEASE="dlb_linux_src_release_7.5.0_2022_01_13.txz"
ARG DLB_DRIVER_SHA256="ae6895ce961c331ead44982dca11e931012da8efb6ed1e8309f3af860262bf62"

RUN wget https://downloadmirror.intel.com/713567/$DLB_DRIVER_RELEASE \
    && echo "$DLB_DRIVER_SHA256 $DLB_DRIVER_RELEASE" | sha256sum -c - \
    && tar -xvf *.txz --no-same-owner

# Build libdlb
RUN cd dlb/libdlb && make

FROM ubuntu:18.04
COPY --from=builder /dlb-build/dlb/libdlb/libdlb.so /usr/local/lib
RUN ldconfig

COPY --from=builder /dlb-build/dlb/libdlb/examples/*traffic /usr/local/bin/
COPY test.sh /usr/bin/

ENTRYPOINT /usr/bin/test.sh
