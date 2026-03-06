FROM ubuntu:24.04 AS builder

RUN apt update && \
    env DEBIAN_FRONTEND=noninteractive apt install -y \
    build-essential \
    curl \
    libcurl4-openssl-dev

WORKDIR /opt/intel

ARG SGX_SDK_URL=https://download.01.org/intel-sgx/sgx-linux/2.27/distro/ubuntu24.04-server/sgx_linux_x64_sdk_2.27.100.1.bin

RUN curl -sSLfO ${SGX_SDK_URL} \
 && export SGX_SDK_INSTALLER=$(basename $SGX_SDK_URL) \
 && chmod +x $SGX_SDK_INSTALLER \
 && ./$SGX_SDK_INSTALLER --prefix /opt/intel \
 && rm $SGX_SDK_INSTALLER

ARG DCAP_VERSION=DCAP_1.24
ARG DCAP_TARBALL_SHA256="c9295f5fd3f489b2fbd5f0d33836b09420976506ac834bc9c9a401f4a6a1204a"

RUN curl -sSLfO https://github.com/intel/confidential-computing.tee.dcap/archive/$DCAP_VERSION.tar.gz && \
    echo "$DCAP_TARBALL_SHA256 $DCAP_VERSION.tar.gz" | sha256sum -c - && \
    tar xzf $DCAP_VERSION.tar.gz && mv confidential-computing.tee.dcap-* SGXDataCenterAttestationPrimitives

WORKDIR SGXDataCenterAttestationPrimitives/tools/PCKRetrievalTool

RUN sed -e 's:sys/firmware/efi:run:g' -i App/utility.cpp \
    && make

FROM ubuntu:24.04

WORKDIR /opt/intel/sgx-pck-id-retrieval-tool/
COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/tools/PCKRetrievalTool/PCKIDRetrievalTool .

RUN ln -sf /lib/x86_64-linux-gnu/libsgx_id_enclave.signed.so.1 && \
    ln -sf /lib/x86_64-linux-gnu/libsgx_pce.signed.so.1

ARG SGX_SDK_VERSION=2_27_100
RUN apt update && apt install -y --no-install-recommends curl ca-certificates gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu noble main" | \
    tee -a /etc/apt/sources.list.d/intel-sgx.list \
    && curl -s https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
    gpg --dearmor --output /usr/share/keyrings/intel-sgx.gpg \
    && curl -sSLf https://download.01.org/intel-sgx/sgx_repo/ubuntu/apt_preference_files/99sgx_${SGXSDK_VERSION}_noble_custom_version.cfg | \
    tee -a /etc/apt/preferences.d/99sgx_sdk \
    && apt update \
    && apt install -y --no-install-recommends \
       tdx-qgs \
       libsgx-ae-pce \
       libsgx-ae-id-enclave \
       libsgx-ra-uefi \
       libsgx-dcap-default-qpl

RUN rm /etc/qgs.conf

COPY dcap-registration-flow /usr/bin

ENTRYPOINT ["/opt/intel/tdx-qgs/qgs", "--no-daemon", "-n=4"]
