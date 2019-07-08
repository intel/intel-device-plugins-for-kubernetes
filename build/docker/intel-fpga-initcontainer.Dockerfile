FROM clearlinux:base as builder

# Move to latest Clear Linux release
# ARG swupd_args
# RUN swupd update --no-boot-update $swupd_args

# Fetch dependencies and source code
ARG OPAE_RElEASE=1.3.2-1

RUN swupd bundle-add wget c-basic go-basic devpkg-json-c devpkg-util-linux devpkg-hwloc doxygen Sphinx && \
    mkdir -p /usr/src/opae && \
    cd /usr/src/opae && \
    wget https://github.com/OPAE/opae-sdk/archive/${OPAE_RElEASE}.tar.gz && \
    tar xf *.tar.gz

# Build OPAE
RUN cd /usr/src/opae/opae-sdk-${OPAE_RElEASE} && \
    mkdir build && \
    cd build && \
    cmake .. -DBUILD_ASE=0 -DCMAKE_SKIP_BUILD_RPATH=1 -DCMAKE_INSTALL_PREFIX=/opt/intel/fpga-sw/opae && \
    make xfpga board_rc fpgaconf fpgainfo

# Install clean os-core and rsync bundle in target directory
RUN source /usr/lib/os-release \
    && mkdir /install_root \
    && swupd os-install -V ${VERSION_ID} \
    --path /install_root --statedir /swupd-state \
    --bundles=os-core,rsync --no-scripts \
    && rm -rf /install_root/var/lib/swupd/*

# Build CRI Hook
ARG DIR=/go/src/github.com/intel/intel-device-plugins-for-kubernetes
WORKDIR $DIR
COPY . .
RUN cd cmd/fpga_crihook && go install && chmod a+x /go/bin/fpga_crihook

# Minimal result image
FROM scratch as final
COPY --from=builder /install_root /

ARG SRC_DIR=/opt/intel/fpga-sw.src
ARG DST_DIR=/opt/intel/fpga-sw

# OPAE
# fpgaconf and fpgainfo
COPY --from=builder /usr/src/opae/opae-sdk-*/build/bin/fpga* $SRC_DIR/opae/bin/
# libxfpga.so, libboard_rc.so, libopae-c.so*, libbitstream.so*
COPY --from=builder /usr/src/opae/opae-sdk-*/build/lib/lib*.so* $SRC_DIR/opae/lib/
COPY --from=builder /usr/lib64/libjson-c.so* $SRC_DIR/opae/lib/
COPY --from=builder /usr/lib64/libuuid.so* $SRC_DIR/opae/lib/

RUN echo -e "#!/bin/sh\n\
export LD_LIBRARY_PATH=$DST_DIR/opae/lib\n\
$DST_DIR/opae/bin/fpgaconf \"\$@\"">> $SRC_DIR/opae/fpgaconf-wrapper && \
    echo -e "#!/bin/sh\n\
export LD_LIBRARY_PATH=$DST_DIR/opae/lib\n\
$DST_DIR/opae/bin/fpgainfo \"\$@\"">> $SRC_DIR/opae/fpgainfo-wrapper && \
    chmod +x $SRC_DIR/opae/*-wrapper


# CRI hook
ARG CRI_HOOK=intel-fpga-crihook
ARG CRI_HOOK_SRC=$SRC_DIR/$CRI_HOOK
ARG CRI_HOOK_DST=$DST_DIR/$CRI_HOOK
ARG HOOK_CONF=$CRI_HOOK.json
ARG HOOK_CONF_SRC=$SRC_DIR/$HOOK_CONF
ARG HOOK_CONF_DST=$DST_DIR/$HOOK_CONF

COPY --from=builder /go/bin/fpga_crihook $CRI_HOOK_SRC

RUN echo -e "#!/bin/sh\n\
rsync -a --delete $SRC_DIR/ $DST_DIR\n\
mkdir -p /etc/containers/oci/hooks.d\n\
ln -sf $HOOK_CONF_DST /etc/containers/oci/hooks.d/$HOOK_CONF\n\
rm $DST_DIR/deploy.sh\n\
">> $SRC_DIR/deploy.sh && chmod +x $SRC_DIR/deploy.sh

CMD [ "/opt/intel/fpga-sw.src/deploy.sh" ]