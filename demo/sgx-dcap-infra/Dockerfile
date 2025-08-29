FROM ubuntu:24.04 AS builder

RUN apt-get update && \
    env DEBIAN_FRONTEND=noninteractive apt-get install -y \
    build-essential \
    curl \
    libcurl4-openssl-dev

WORKDIR /opt/intel

ARG SGX_SDK_URL=https://download.01.org/intel-sgx/sgx-linux/2.26/distro/ubuntu24.04-server/sgx_linux_x64_sdk_2.26.100.0.bin

RUN curl -sSLfO ${SGX_SDK_URL} \
 && export SGX_SDK_INSTALLER=$(basename $SGX_SDK_URL) \
 && chmod +x $SGX_SDK_INSTALLER \
 && echo "yes" | ./$SGX_SDK_INSTALLER \
 && rm $SGX_SDK_INSTALLER

ARG DCAP_VERSION=DCAP_1.23
ARG DCAP_TARBALL_SHA256="c4567e7bc0a2f0dbb70fa2625a9af492e00b96e83d07fa69b9f4f304a9992495"

RUN curl -sSLfO https://github.com/intel/SGXDataCenterAttestationPrimitives/archive/$DCAP_VERSION.tar.gz && \
    echo "$DCAP_TARBALL_SHA256 $DCAP_VERSION.tar.gz" | sha256sum -c - && \
    tar xzf $DCAP_VERSION.tar.gz && mv SGXDataCenterAttestationPrimitives* SGXDataCenterAttestationPrimitives

WORKDIR SGXDataCenterAttestationPrimitives/tools/PCKRetrievalTool

RUN sed -e 's:sys/firmware/efi:run:g' -i App/utility.cpp \
    && make

FROM ubuntu:24.04

WORKDIR /opt/intel/sgx-pck-id-retrieval-tool/
COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/tools/PCKRetrievalTool/PCKIDRetrievalTool .

RUN ln -sf /lib/x86_64-linux-gnu/libsgx_id_enclave.signed.so.1 && \
    ln -sf /lib/x86_64-linux-gnu/libsgx_pce.signed.so.1

ARG SGX_SDK_VERSION=2_26_100
RUN apt update && apt install -y curl gnupg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu noble main" | \
    tee -a /etc/apt/sources.list.d/intel-sgx.list \
    && curl -s https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
    gpg --dearmor --output /usr/share/keyrings/intel-sgx.gpg \
    && curl -sFLf https://download.01.org/intel-sgx/sgx_repo/ubuntu/apt_preference_files/99sgx_${SGXSDK_VERSION}_noble_custom_version.cfg | \
    tee -a /etc/apt/preferences.d/99sgx_sdk \
    && apt update \
    && apt install -y --no-install-recommends \
       libcurl4 \
       tdx-qgs \
       libsgx-ae-pce \
       libsgx-ae-id-enclave \
       libsgx-ra-uefi \
       libsgx-dcap-default-qpl

# BUG: "qgs -p=0" gets overriden by the config file making the parameter useless
RUN sed -e 's/\(^port =\).*/\1 0/g' -i /etc/qgs.conf

COPY dcap-registration-flow /usr/bin

ENTRYPOINT ["/opt/intel/tdx-qgs/qgs", "--no-daemon", "-p=0"]
