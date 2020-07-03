# CLEAR_LINUX_BASE and CLEAR_LINUX_VERSION can be used to make the build
# reproducible by choosing an image by its hash and installing an OS version
# with --version=:
# CLEAR_LINUX_BASE=clearlinux@sha256:b8e5d3b2576eb6d868f8d52e401f678c873264d349e469637f98ee2adf7b33d4
# CLEAR_LINUX_VERSION="--version=29970"
#
# This is used on release branches before tagging a stable version.
# The master branch defaults to using the latest Clear Linux.
ARG CLEAR_LINUX_BASE=clearlinux/golang@sha256:9f04d3cc0ca3f6951ab3646639b43eb73e963a7cee7322d619a02c7eeecce711

FROM ${CLEAR_LINUX_BASE} as builder

ARG CLEAR_LINUX_VERSION="--version=33450"

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
    --bundles=rsync \
    --no-boot-update \
    && rm -rf /install_root/var/lib/swupd/*

# Build CRI Hook
RUN cd $DIR/cmd/fpga_crihook && \
    GO111MODULE=${GO111MODULE} go install && \
    chmod a+x /go/bin/fpga_crihook && \
    cd $DIR/cmd/fpga_tool && \
    go install && \
    chmod a+x /go/bin/fpga_tool && \
    cd $DIR && \
    install -D ${DIR}/LICENSE /install_root/usr/local/share/package-licenses/intel-device-plugins-for-kubernetes/LICENSE && \
    scripts/copy-modules-licenses.sh ./cmd/fpga_crihook /install_root/usr/local/share/package-licenses/ && \
    scripts/copy-modules-licenses.sh ./cmd/fpga_tool /install_root/usr/local/share/package-licenses/

# Minimal result image
FROM scratch as final
COPY --from=builder /install_root /

ARG SRC_DIR=/usr/local/fpga-sw
ARG DST_DIR=/opt/intel/fpga-sw

# CRI hook
ARG CRI_HOOK=intel-fpga-crihook
ARG FPGA_TOOL=fpgatool
ARG HOOK_CONF=$CRI_HOOK.json
ARG HOOK_CONF_SRC=$SRC_DIR/$HOOK_CONF
ARG HOOK_CONF_DST=$DST_DIR/$HOOK_CONF

COPY --from=builder /go/bin/fpga_crihook $SRC_DIR/$CRI_HOOK
COPY --from=builder /go/bin/fpga_tool $SRC_DIR/$FPGA_TOOL

RUN echo -e "{\n\
    \"hook\" : \"$DST_DIR/$CRI_HOOK\",\n\
    \"stage\" : [ \"prestart\" ],\n\
    \"annotation\": [ \"fpga.intel.com/region\" ]\n\
}\n">>$HOOK_CONF_SRC

RUN echo -e "#!/bin/sh\n\
rsync -a --delete $SRC_DIR/ $DST_DIR\n\
mkdir -p /etc/containers/oci/hooks.d\n\
ln -sf $HOOK_CONF_DST /etc/containers/oci/hooks.d/$HOOK_CONF\n\
rm $DST_DIR/deploy.sh\n\
">> $SRC_DIR/deploy.sh && chmod +x $SRC_DIR/deploy.sh

ENTRYPOINT [ "/usr/local/fpga-sw/deploy.sh" ]
