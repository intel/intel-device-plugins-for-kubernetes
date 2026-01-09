## This is a generated file, do not edit directly. Edit build/docker/templates/intel-idxd-config-initcontainer.Dockerfile.in instead.
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
ARG FINAL_BASE_DYN=registry.access.redhat.com/ubi9/ubi:latest
FROM ${FINAL_BASE_DYN}
ARG UBI
COPY ./LICENSE /licenses/intel-device-plugins-for-kubernetes/LICENSE
RUN if [ $UBI -eq 0 ]; then \
    echo "deb-src http://deb.debian.org/debian unstable main" >> \
    /etc/apt/sources.list.d/deb-src.list; \
    apt-get update && apt-get install -y --no-install-recommends accel-config jq && rm -rf /var/lib/apt/lists/\*; \
    else \
    dnf install --setopt=install_weak_deps=False --setopt=tsflags=nodocs -y accel-config jq && dnf -y autoremove && \
    dnf clean all && rm -rf /var/cache/dnf; fi
RUN cd /licenses/intel-device-plugins-for-kubernetes
RUN if [ $UBI -eq 0 ]; then apt-get source --download-only -y accel-config; \
    else dnf install -y dnf-plugins-core && dnf download --source accel-config && \
    dnf remove -y dnf-plugins-core && dnf -y autoremove && dnf clean all && rm -rf /var/cache/dnf; fi
COPY demo/idxd-init.sh /usr/local/bin/
COPY demo/dsa.conf /idxd-init/
COPY demo/iaa.conf /idxd-init/
RUN mkdir /idxd-init/scratch
WORKDIR /idxd-init
ENTRYPOINT ["/usr/local/bin/idxd-init.sh"]
