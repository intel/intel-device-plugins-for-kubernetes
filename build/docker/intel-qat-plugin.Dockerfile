# CLEAR_LINUX_BASE and CLEAR_LINUX_VERSION can be used to make the build
# reproducible by choosing an image by its hash and installing an OS version
# with --version=:
# CLEAR_LINUX_BASE=clearlinux@sha256:b8e5d3b2576eb6d868f8d52e401f678c873264d349e469637f98ee2adf7b33d4
# CLEAR_LINUX_VERSION="--version=29970"
#
# This is used on release branches before tagging a stable version.
# The master branch defaults to using the latest Clear Linux.
ARG CLEAR_LINUX_BASE=clearlinux/golang@sha256:88c32c98c72dab8f5a0760e48c0121d11ee8a962b13b10b362acaa63879a49cb

FROM ${CLEAR_LINUX_BASE} as builder

ARG CLEAR_LINUX_VERSION="--version=32510"

RUN swupd update --no-boot-update ${CLEAR_LINUX_VERSION}

ARG DIR=/intel-device-plugins-for-kubernetes
ARG GO111MODULE=on
WORKDIR $DIR
COPY . .

ARG TAGS_KERNELDRV=

RUN mkdir /install_root \
    && swupd os-install \
    ${CLEAR_LINUX_VERSION} \
    --path /install_root \
    --statedir /swupd-state \
    --bundles=os-core$(test -z "${TAGS_KERNELDRV}" || echo ",libstdcpp") \
    --no-boot-update \
    && rm -rf /install_root/var/lib/swupd/*

ARG QAT_DRIVER_RELEASE="qat1.7.l.4.6.0-00025"

RUN test -z "${TAGS_KERNELDRV}" \
    || ( swupd bundle-add wget c-basic \
    && mkdir -p /usr/src/qat \
    && cd /usr/src/qat  \
    && wget https://01.org/sites/default/files/downloads/${QAT_DRIVER_RELEASE}.tar.gz \
    && tar xf *.tar.gz \
    && cd /usr/src/qat/quickassist/utilities/adf_ctl \
    && make KERNEL_SOURCE_DIR=/usr/src/qat/quickassist/qat \
    && install -D adf_ctl /install_root/usr/local/bin/adf_ctl )
RUN cd cmd/qat_plugin; echo "build tags: ${TAGS_KERNELDRV}"; GO111MODULE=${GO111MODULE} go install -tags "${TAGS_KERNELDRV}"; cd -
RUN chmod a+x /go/bin/qat_plugin \
    && install -D /go/bin/qat_plugin /install_root/usr/local/bin/intel_qat_device_plugin \
    && install -D ${DIR}/LICENSE /install_root/usr/local/share/package-licenses/intel-device-plugins-for-kubernetes/LICENSE \
    && scripts/copy-modules-licenses.sh ./cmd/qat_plugin /install_root/usr/local/share/package-licenses/

FROM scratch as final
COPY --from=builder /install_root /
ENV PATH=/usr/local/bin
ENTRYPOINT ["/usr/local/bin/intel_qat_device_plugin"]
