## This is a generated file, do not edit directly. Edit build/docker/templates/intel-idxd-config-initcontainer.ubi.Dockerfile.in instead.
##
## Copyright 2022 Intel Corporation. All Rights Reserved.
##
## Licensed under the Apache License, Version 2.0 (the "License");
## you may not use this file except in compliance with the License.
## You may obtain a copy of the License at
##
## http://www.apache.org/licenses/LICENSE-2.0
##
## Unless required by applicable law or agreed to in writing, software
## distributed under the License is distributed on an "AS IS" BASIS,
## WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
## See the License for the specific language governing permissions and
## limitations under the License.
###
FROM registry.access.redhat.com/ubi9/ubi:latest
COPY ./LICENSE /licenses/intel-device-plugins-for-kubernetes/LICENSE
RUN dnf install --setopt=install_weak_deps=False --setopt=tsflags=nodocs -y accel-config jq && dnf -y autoremove && \
    dnf clean all && rm -rf /var/cache/dnf && \
    cd /licenses/intel-device-plugins-for-kubernetes && \
    dnf install -y dnf-plugins-core && dnf download --source accel-config && \
    dnf remove -y dnf-plugins-core && dnf -y autoremove && dnf clean all && rm -rf /var/cache/dnf
COPY demo/idxd-init.sh /usr/local/bin/
COPY demo/dsa.conf /idxd-init/
COPY demo/iaa.conf /idxd-init/
RUN mkdir /idxd-init/scratch
WORKDIR /idxd-init
ENTRYPOINT ["/usr/local/bin/idxd-init.sh"]
LABEL name='intel-idxd-config-initcontainer'
LABEL summary='Intel® IDXD config initcontainer for Kubernetes'
LABEL description='IDXD config configures DSA and IAA devices for use with the DSA/IAA plugin'
LABEL vendor='Intel®'
LABEL org.opencontainers.image.source='https://github.com/intel/intel-device-plugins-for-kubernetes'
LABEL maintainer="Intel®"
LABEL version='devel'
LABEL release='1'
