FROM centos:7

ENV SHELL=/bin/bash

ENV RTE_TARGET=x86_64-native-linuxapp-gcc

# verified git commit hash
ENV DPDK_VERSION=3d2e044

# install dependencies and some handy tools
RUN rpm --rebuilddb; yum -y clean all; yum -y update;
RUN yum -y install curl pciutils git numactl-devel make gcc openssl-devel

# clone DPDK and checkout desired version
WORKDIR /usr/src
RUN git clone http://dpdk.org/git/dpdk
WORKDIR /usr/src/dpdk
RUN git checkout $DPDK_VERSION

# explicitly enable QAT in the config
RUN sed s#CONFIG_RTE_LIBRTE_PMD_QAT=n#CONFIG_RTE_LIBRTE_PMD_QAT=y# \
        -i config/common_base

# don't build kernel modules as they depend on the host kernel version
RUN sed s#CONFIG_RTE_EAL_IGB_UIO=y#CONFIG_RTE_EAL_IGB_UIO=n# \
        -i config/common_linuxapp
RUN sed s#CONFIG_RTE_KNI_KMOD=y#CONFIG_RTE_KNI_KMOD=n# \
        -i config/common_linuxapp

# build DPDK
RUN make T=$RTE_TARGET config && \
    make T=$RTE_TARGET && \
    make T=$RTE_TARGET DESTDIR=install install

# go to the location of dpdk-test-crypto-perf app by default
WORKDIR /usr/src/dpdk/build/app
