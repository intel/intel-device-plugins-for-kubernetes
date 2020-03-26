# CLEAR_LINUX_BASE and CLEAR_LINUX_VERSION can be used to make the build
# reproducible by choosing an image by its hash and installing an OS version
# with --version=:
# CLEAR_LINUX_BASE=clearlinux@sha256:b8e5d3b2576eb6d868f8d52e401f678c873264d349e469637f98ee2adf7b33d4
# CLEAR_LINUX_VERSION="--version=29970"
#
# This is used on release branches before tagging a stable version.
# The master branch defaults to using the latest Clear Linux.
ARG CLEAR_LINUX_BASE=clearlinux/golang:latest

FROM ${CLEAR_LINUX_BASE} as builder

ARG CLEAR_LINUX_VERSION=

RUN swupd update --no-boot-update ${CLEAR_LINUX_VERSION}

ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
WORKDIR $DIR
COPY . .

RUN mkdir /install_root \
    && swupd os-install \
    ${CLEAR_LINUX_VERSION} \
    --path /install_root \
    --statedir /swupd-state \
    --no-boot-update \
    && rm -rf /install_root/var/lib/swupd/*

RUN cd cmd/sgx_plugin; GO111MODULE=${GO111MODULE} go install; cd -
RUN chmod a+x /go/bin/sgx_plugin \
    && install -D /go/bin/sgx_plugin /install_root/usr/local/bin/intel_sgx_device_plugin \
    && install -D ${DIR}/LICENSE /install_root/usr/local/share/package-licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && scripts/copy-modules-licenses.sh ./cmd/sgx_plugin /install_root/usr/local/share/package-licenses/

FROM scratch as final
COPY --from=builder /install_root /
ENTRYPOINT ["/usr/local/bin/intel_sgx_device_plugin"]
