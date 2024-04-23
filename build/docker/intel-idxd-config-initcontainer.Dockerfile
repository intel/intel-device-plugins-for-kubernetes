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
FROM debian:stable as build
RUN apt-get update && apt-get install -y --no-install-recommends accel-config jq curl ca-certificates make libc6-dev && rm -rf /var/lib/apt/lists/\*
RUN mkdir -p /idxd-init/scratch
ARG DIR=/intel-device-plugins-for-kubernetes
WORKDIR ${DIR}
COPY . .
ARG TOYBOX_VERSION="0.8.11"
ARG TOYBOX_SHA256="83a3a88cbe1fa30f099c2f58295baef4637aaf988085aaea56e03aa29168175d"
ARG ROOT=/install_root
RUN apt-get update && apt-get --no-install-recommends -y install musl musl-tools musl-dev
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
ARG FINAL_BASE=registry.access.redhat.com/ubi9-micro:latest
RUN curl -SL https://github.com/landley/toybox/archive/refs/tags/$TOYBOX_VERSION.tar.gz -o toybox.tar.gz \
    && echo "$TOYBOX_SHA256 toybox.tar.gz" | sha256sum -c - \
    && tar -xzf toybox.tar.gz \
    && rm toybox.tar.gz \
    && cd toybox-$TOYBOX_VERSION \
    && KCONFIG_CONFIG=${DIR}/build/docker/toybox-config-$(echo ${FINAL_BASE} | xargs basename -s :latest) LDFLAGS="--static" CC=musl-gcc PREFIX=$ROOT/usr/bin V=2 make toybox install_flat \
    && install -D LICENSE $ROOT/licenses/toybox \
    && cp -r /usr/share/doc/musl $ROOT/licenses/
###
ARG LIBS="libjson-c libaccel-config libjq libtinfo libonig"
ARG EXECS="accel-config jq"
ARG LICENSES="jq libjq1 libjson-c5 libaccel-config1 libonig5 libtinfo6"
RUN mkdir /tmp/libs && for l in ${LIBS}; do cp "/lib/x86_64-linux-gnu/${l}.so"* /tmp/libs/; done
RUN mkdir /tmp/bins && for b in ${EXECS}; do cp "/usr/bin/${b}" /tmp/bins/; done
RUN mkdir /tmp/licenses && for l in ${LICENSES}; do cp -r "/usr/share/doc/${l}" /tmp/licenses/; done
FROM gcr.io/distroless/cc
COPY ./LICENSE /licenses/intel-device-plugins-for-kubernetes/LICENSE
COPY demo/idxd-init.sh /usr/local/bin/
COPY demo/dsa.conf /idxd-init/
COPY demo/iaa.conf /idxd-init/
COPY --from=build /idxd-init /idxd-init
COPY --from=build /install_root /
COPY --from=build /tmp/bins//* /usr/bin/
COPY --from=build /tmp/libs//* /lib/x86_64-linux-gnu/
COPY --from=build /tmp/licenses/ /usr/share/doc/
WORKDIR /idxd-init
ENTRYPOINT ["/usr/local/bin/idxd-init.sh"]
