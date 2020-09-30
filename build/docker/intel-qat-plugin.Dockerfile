# CLEAR_LINUX_BASE and CLEAR_LINUX_VERSION can be used to make the build
# reproducible by choosing an image by its hash and installing an OS version
# with --version=:
# CLEAR_LINUX_BASE=clearlinux@sha256:b8e5d3b2576eb6d868f8d52e401f678c873264d349e469637f98ee2adf7b33d4
# CLEAR_LINUX_VERSION="--version=29970"
#
# This is used on release branches before tagging a stable version.
# The master branch defaults to using the latest Clear Linux.
ARG CLEAR_LINUX_BASE=clearlinux/golang@sha256:7f790763c87853f6e553f7317101d6e5eb337b7d0454c081d40890b5f062de4a

FROM ${CLEAR_LINUX_BASE} as builder

ARG CLEAR_LINUX_VERSION="--version=33720"

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
    $(test -z "${TAGS_KERNELDRV}" || echo "--bundles=libstdcpp") \
    --no-boot-update \
    && rm -rf /install_root/var/lib/swupd/*

ARG QAT_DRIVER_RELEASE="qat1.7.l.4.8.0-00005"

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
    && scripts/copy-modules-licenses.sh ./cmd/qat_plugin /install_root/usr/local/share/

FROM scratch as final
COPY --from=builder /install_root /
ENV PATH=/usr/local/bin
ENTRYPOINT ["/usr/local/bin/intel_qat_device_plugin"]
