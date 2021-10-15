FROM ubuntu:bionic AS builder

WORKDIR /root

RUN apt-get update && \
    apt-get install -y \
    wget \
    unzip \
    protobuf-compiler \
    libprotobuf-dev \
    build-essential \
    cmake \
    pkg-config \
    gdb \
    vim \
    python3 \
    git \
    gnupg \
 && apt-get -y -q upgrade \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# SGX SDK is installed in /opt/intel directory.
WORKDIR /opt/intel

ARG SGX_SDK_INSTALLER=sgx_linux_x64_sdk_2.15.100.3.bin
ARG DCAP_VERSION=DCAP_1.12

RUN echo "deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu bionic main" >> /etc/apt/sources.list.d/intel-sgx.list \
 && wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add - \
 && apt-get update \
 && apt-get install -y \
    libsgx-dcap-ql-dev \
    libsgx-dcap-default-qpl-dev \
    libsgx-quote-ex-dev

# Install SGX SDK
RUN wget https://download.01.org/intel-sgx/sgx-linux/2.15/distro/ubuntu18.04-server/$SGX_SDK_INSTALLER \
 && chmod +x  $SGX_SDK_INSTALLER \
 && echo "yes" | ./$SGX_SDK_INSTALLER \
 && rm $SGX_SDK_INSTALLER

RUN git clone -b $DCAP_VERSION https://github.com/intel/SGXDataCenterAttestationPrimitives.git

RUN cd sgxsdk/SampleCode/SampleEnclave \
    && . /opt/intel/sgxsdk/environment \
    && make \
    && cd -

RUN cd SGXDataCenterAttestationPrimitives/SampleCode/QuoteGenerationSample \
    && . /opt/intel/sgxsdk/environment \
    && make \
    && cd -

FROM ubuntu:bionic

RUN apt-get update && \
    apt-get install -y \
    wget \
    gnupg

# Add 01.org to apt for SGX packages and install SGX runtime components
RUN echo "deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu bionic main" >> /etc/apt/sources.list.d/intel-sgx.list \
 && wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add - \
 && apt-get update \
 && apt-get install -y --no-install-recommends \
    libsgx-enclave-common \
    libsgx-urts \
    libsgx-quote-ex \
    libsgx-dcap-ql \
    libsgx-dcap-default-qpl \
 && mkdir -p /opt/intel/sgx-sample-app/ \
 && mkdir -p /opt/intel/sgx-quote-generation/

COPY --from=builder /opt/intel/sgxsdk/SampleCode/SampleEnclave/app /opt/intel/sgx-sample-app/sgx-sample-app
COPY --from=builder /opt/intel/sgxsdk/SampleCode/SampleEnclave/enclave.signed.so /opt/intel/sgx-sample-app/enclave.signed.so

COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/SampleCode/QuoteGenerationSample/app /opt/intel/sgx-quote-generation/sgx-quote-generation
COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/SampleCode/QuoteGenerationSample/enclave.signed.so /opt/intel/sgx-quote-generation/enclave.signed.so

ENTRYPOINT /opt/intel/sgx-sample-app/sgx-sample-app
