FROM registry.access.redhat.com/ubi9/nodejs-20:latest AS builder

USER root

RUN dnf install -y \
        file \
        jq \
        zip

ARG DCAP_VERSION=DCAP_1.23

RUN git clone -b ${DCAP_VERSION} --depth 1 --recurse-submodules https://github.com/intel/SGXDataCenterAttestationPrimitives.git

WORKDIR SGXDataCenterAttestationPrimitives/tools/PCKCertSelection/
RUN mkdir -p ../../prebuilt/openssl/inc \
    && mkdir -p ../../QuoteGeneration/pccs/lib \
    && make \
    && cp ./out/libPCKCertSelection.so ../../QuoteGeneration/pccs/lib

WORKDIR /opt/app-root/src/SGXDataCenterAttestationPrimitives/QuoteGeneration/pccs
RUN npm config set engine-strict true \
    && npm install \
    && npm audit fix

FROM registry.access.redhat.com/ubi9/nodejs-20-minimal:latest

WORKDIR intel/pccs
COPY --from=builder --chown=1001:users /opt/app-root/src/SGXDataCenterAttestationPrimitives/QuoteGeneration/pccs .
COPY --chown=1001:users default.json config/
COPY --chown=1001:users custom-environment-variables.json config/

ENTRYPOINT ["/usr/bin/node", "pccs_server.js"]
