FROM ubuntu:20.04 AS builder

WORKDIR /dlb-build

# Install build dependencies
RUN apt-get update && apt-get install -y wget xz-utils make gcc

# Download and unpack DLB Driver tarball
ARG DLB_DRIVER_RELEASE="dlblinuxsrcrelease7.4.020211006"
ARG DLB_DRIVER_SHA256="715c34314d77dce9fe0cd61d5c0f269016971e8d9aa680bc0e4a32d4284aae09"

RUN wget https://downloadmirror.intel.com/690271/$DLB_DRIVER_RELEASE.txz \
    && echo "$DLB_DRIVER_SHA256 $DLB_DRIVER_RELEASE.txz" | sha256sum -c - \
    && tar -xvf *.txz --no-same-owner

# Build libdlb
RUN cd dlb/libdlb && make

FROM ubuntu:20.04
COPY --from=builder /dlb-build/dlb/libdlb/libdlb.so /usr/local/lib
RUN ldconfig

COPY --from=builder /dlb-build/dlb/libdlb/examples/*traffic /usr/local/bin/
COPY test.sh /usr/bin/

ENTRYPOINT /usr/bin/test.sh
