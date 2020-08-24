FROM ubuntu:bionic AS builder

WORKDIR /root

RUN apt-get update && \
    apt-get install -y \
    wget \
    unzip \
    curl \
    protobuf-compiler \
    libprotobuf-dev \
    build-essential \
    cmake \
    pkg-config \
    gdb \
    vim \
    python3 \
 && apt-get -y -q upgrade \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# SGX SDK is installed in /opt/intel directory.
WORKDIR /opt/intel

ARG SGX_SDK_INSTALLER=sgx_linux_x64_sdk_2.10.100.2.bin

# Install SGX SDK
RUN wget https://download.01.org/intel-sgx/sgx-linux/2.10/distro/ubuntu18.04-server/$SGX_SDK_INSTALLER \
 && chmod +x  $SGX_SDK_INSTALLER \
 && echo "yes" | ./$SGX_SDK_INSTALLER \
 && rm $SGX_SDK_INSTALLER

RUN cd sgxsdk/SampleCode/SampleEnclave \
    && . /opt/intel/sgxsdk/environment \
    && make

FROM ubuntu:bionic

RUN apt-get update && \
    apt-get install -y \
    wget \
    gnupg

# Add 01.org to apt for SGX packages and install SGX runtime components
RUN echo "deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu bionic main" >> /etc/apt/sources.list.d/intel-sgx.list \
 && wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add - \
 && apt-get update \
 && apt-get install -y libsgx-enclave-common libsgx-urts \
 && mkdir -p /opt/intel/sgx-sample-app/

COPY --from=builder /opt/intel/sgxsdk/SampleCode/SampleEnclave/app /opt/intel/sgx-sample-app/sgx-sample-app
COPY --from=builder /opt/intel/sgxsdk/SampleCode/SampleEnclave/enclave.signed.so /opt/intel/sgx-sample-app/enclave.signed.so
COPY --from=builder /opt/intel/sgxsdk/SampleCode/SampleEnclave/enclave.so /opt/intel/sgx-sample-app/enclave.so

WORKDIR /opt/intel/sgx-sample-app/

ENTRYPOINT /opt/intel/sgx-sample-app/sgx-sample-app
