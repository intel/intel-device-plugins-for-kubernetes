### 1. Temporary image to prepare AOCL and OPAE components
FROM centos:centos7.4.1708 as temporary

# install aocl and opae deps
RUN rpm --import /etc/pki/rpm-gpg/RPM-GPG-KEY-CentOS-7
RUN yum -y install which perl uuid libuuid-devel json-c

ADD a10_gx_pac_ias_1_1_pv_rte_installer.tar.gz $WORKDIR

# install opae
RUN tar -zxf a10_gx_pac_ias_1_1_pv_rte_installer/components/a10_gx_pac_ias_1_1_pv.tar.gz sw
RUN rpm -ihv sw/opae-libs*.rpm sw/opae-tools*.rpm sw/opae-devel*.rpm

# install aocl rte
RUN a10_gx_pac_ias_1_1_pv_rte_installer/components/aocl-pro-rte-17.1.1.273-linux.run --mode unattended --installdir / --accept_eula 1

# unpack opencl_bsp
RUN tar -zxf a10_gx_pac_ias_1_1_pv_rte_installer/components/a10_gx_pac_ias_1_1_pv.tar.gz opencl/opencl_bsp.tar.gz
RUN tar -zxf opencl/opencl_bsp.tar.gz && rm -rf opencl_bsp/hardware

### 2. Final initcontainer image
FROM alpine as final

ARG SRC_DIR=/opt/intel/fpga-sw.src
ARG DST_DIR=/opt/intel/fpga-sw

# OpenCL
COPY --from=temporary aclrte-linux64 $SRC_DIR/opencl/aclrte-linux64
COPY --from=temporary opencl_bsp $SRC_DIR/opencl/opencl_bsp

RUN echo -e "#!/bin/sh\n\
export INTEL_FPGA_ROOT=$DST_DIR\n\
export FPGA_OPENCL_ROOT=\$INTEL_FPGA_ROOT/opencl\n\
export AOCL_BOARD_PACKAGE_ROOT=\$FPGA_OPENCL_ROOT/opencl_bsp\n\
export INTELFPGAOCLSDKROOT=\$FPGA_OPENCL_ROOT/aclrte-linux64\n\
export LD_LIBRARY_PATH=\$AOCL_BOARD_PACKAGE_ROOT/linux64/lib:\$INTEL_FPGA_ROOT/opae/lib\n\
\$INTELFPGAOCLSDKROOT/bin/aocl \"\$@\"" >> $SRC_DIR/opencl/aocl-wrapper
RUN chmod +x $SRC_DIR/opencl/aocl-wrapper

# OPAE
COPY --from=temporary /usr/bin/fpgaconf $SRC_DIR/opae/bin/
COPY --from=temporary /usr/bin/packager $SRC_DIR/opae/bin/
COPY --from=temporary /usr/lib64/libopae-c.so* $SRC_DIR/opae/lib/
COPY --from=temporary /usr/lib64/libjson-c.so* $SRC_DIR/opae/lib/
COPY --from=temporary /usr/lib64/libuuid.so* $SRC_DIR/opae/lib/

RUN echo -e "#!/bin/sh\n\
export LD_LIBRARY_PATH=$DST_DIR/opae/lib\n\
$DST_DIR/opae/bin/fpgaconf \"\$@\"">> $SRC_DIR/opae/fpgaconf-wrapper
RUN chmod +x $SRC_DIR/opae/fpgaconf-wrapper

# CRI hook

ARG CRI_HOOK=intel-fpga-crihook
ARG CRI_HOOK_SRC=$SRC_DIR/$CRI_HOOK
ARG CRI_HOOK_DST=$DST_DIR/$CRI_HOOK
ARG HOOK_CONF=$CRI_HOOK.json
ARG HOOK_CONF_SRC=$SRC_DIR/$HOOK_CONF
ARG HOOK_CONF_DST=$DST_DIR/$HOOK_CONF

COPY ./$CRI_HOOK $CRI_HOOK_SRC

RUN echo -e "{\n\
    \"hook\" : \"$CRI_HOOK_DST\",\n\
    \"stage\" : [ \"prestart\" ],\n\
    \"annotation\": [ \"fpga.intel.com/region\" ]\n\
}\n">>$HOOK_CONF_SRC

# Setup

RUN apk update
RUN apk add rsync

RUN echo -e "#!/bin/sh\n\
rsync -a --delete $SRC_DIR/ $DST_DIR\n\
ln -sf $HOOK_CONF_DST /etc/containers/oci/hooks.d/$HOOK_CONF\n\
rm $DST_DIR/deploy.sh\n\
">> $SRC_DIR/deploy.sh
RUN chmod +x $SRC_DIR/deploy.sh
