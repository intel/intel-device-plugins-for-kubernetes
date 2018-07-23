FROM centos:7
ENV SHELL=/bin/bash
COPY ./resolv.conf /etc/resolv.conf
RUN rpm --rebuilddb; yum -y clean all; yum -y update; RUN yum -y install epel-release  httpd httpd-tools curl traceroute tcpdump bind-utils nc net-tools nmap screen htop cairo-devel hwloc-libs hwloc-gui pciutils
RUN mv /usr/sbin/tcpdump /usr/bin/tcpdump
WORKDIR /usr/src/dpdk
COPY . /usr/src/dpdk
WORKDIR /usr/src/dpdk/build/app
