# CLEAR_LINUX_BASE and CLEAR_LINUX_VERSION can be used to make the build
# reproducible by choosing an image by its hash and installing an OS version
# with --version=:
# CLEAR_LINUX_BASE=clearlinux@sha256:b8e5d3b2576eb6d868f8d52e401f678c873264d349e469637f98ee2adf7b33d4
# CLEAR_LINUX_VERSION="--version=29970"
#
# This is used on release branches before tagging a stable version.
# The main branch defaults to using the latest Clear Linux.
ARG CLEAR_LINUX_BASE=clearlinux@sha256:2e600dc800b74f3af7ed9d6951340bcd1e4c113d43da001da63e4db7c053b191

FROM ${CLEAR_LINUX_BASE} as builder

ARG CLEAR_LINUX_VERSION="--version=34740"

RUN mkdir /install_root && \
    swupd os-install \
    ${CLEAR_LINUX_VERSION} \
    --path /install_root \
    --statedir /swupd-state \
    --bundles=dpdk \
    --no-boot-update && \
    rm -rf /install_root/var/lib/swupd/*

ADD run-dpdk-test /install_root/usr/bin/run-dpdk-test

FROM scratch as final
COPY --from=builder /install_root /

CMD ["/bin/bash"]
