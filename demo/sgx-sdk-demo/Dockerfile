FROM ubuntu:26.04 AS builder

WORKDIR /root

RUN apt-get update && \
    env DEBIAN_FRONTEND=noninteractive apt-get install -y \
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
    openssl \
 && apt-get -y -q upgrade \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/*

# SGX SDK is installed in /opt/intel directory.
WORKDIR /opt/intel

ARG DCAP_VERSION=DCAP_1.27.101
# Pins the SGX PSW/DCAP packages to the versions matching SGX_SDK_URL and DCAP_VERSION
ARG SGX_APT_PREFERENCES=99sgx_2_30_100_resolute_custom_version.cfg

RUN echo "deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu resolute main" | \
    tee -a /etc/apt/sources.list.d/intel-sgx.list \
 && wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
    gpg --dearmor --output /usr/share/keyrings/intel-sgx.gpg \
 && wget -qO /etc/apt/preferences.d/intel-sgx https://download.01.org/intel-sgx/sgx_repo/ubuntu/apt_preference_files/${SGX_APT_PREFERENCES} \
 && apt-get update \
 && env DEBIAN_FRONTEND=noninteractive apt-get install -y \
    libsgx-dcap-ql-dev \
    libsgx-dcap-quote-verify-dev \
    libsgx-dcap-default-qpl-dev \
    libsgx-quote-ex-dev

# Install SGX SDK
ARG SGX_SDK_URL=https://download.01.org/intel-sgx/sgx-linux/2.30/distro/ubuntu26.04-server/sgx_linux_x64_sdk_2.30.100.1.bin
RUN wget ${SGX_SDK_URL} \
 && export SGX_SDK_INSTALLER=$(basename $SGX_SDK_URL) \
 && chmod +x $SGX_SDK_INSTALLER \
 && echo "yes" | ./$SGX_SDK_INSTALLER \
 && rm $SGX_SDK_INSTALLER

RUN cd sgxsdk/SampleCode/SampleEnclave \
    && . /opt/intel/sgxsdk/environment \
    && make \
    && cd -

ARG DCAP_TARBALL_SHA256="433dd48679131feb7af1783fc6710218806e36332b00115ff09d6397f9cb3620"

RUN wget -q https://github.com/intel/confidential-computing.tee.dcap/archive/$DCAP_VERSION.tar.gz && \
    echo "$DCAP_TARBALL_SHA256 $DCAP_VERSION.tar.gz" | sha256sum -c - && \
    tar xzf $DCAP_VERSION.tar.gz && mv confidential-computing.tee.dcap* SGXDataCenterAttestationPrimitives

RUN cd SGXDataCenterAttestationPrimitives/SampleCode/QuoteGenerationSample \
    && . /opt/intel/sgxsdk/environment \
    && make \
    && cd -

RUN cd SGXDataCenterAttestationPrimitives/SampleCode/QuoteVerificationSample \
    && . /opt/intel/sgxsdk/environment \
    && make HW_RELEASE=1 \
    && cd -

FROM ubuntu:26.04

RUN apt-get update && \
    apt-get install -y \
    wget \
    gnupg

ARG SGX_APT_PREFERENCES=99sgx_2_30_100_resolute_custom_version.cfg

# Add 01.org to apt for SGX packages and install SGX runtime components
RUN echo "deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu resolute main" | \
    tee -a /etc/apt/sources.list.d/intel-sgx.list \
 && wget -qO - https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
    gpg --dearmor --output /usr/share/keyrings/intel-sgx.gpg \
 && wget -qO /etc/apt/preferences.d/intel-sgx https://download.01.org/intel-sgx/sgx_repo/ubuntu/apt_preference_files/${SGX_APT_PREFERENCES} \
 && apt-get update \
 && env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    libsgx-enclave-common \
    libsgx-urts \
    libsgx-quote-ex \
    libsgx-dcap-quote-verify \
    libsgx-ae-qve \
    libsgx-dcap-ql \
    libsgx-dcap-default-qpl \
 && mkdir -p /opt/intel/sgx-sample-app/ \
 && mkdir -p /opt/intel/sgx-quote-verification/ \
 && mkdir -p /opt/intel/sgx-quote-generation/

COPY --from=builder /opt/intel/sgxsdk/SampleCode/SampleEnclave/app /opt/intel/sgx-sample-app/sgx-sample-app
COPY --from=builder /opt/intel/sgxsdk/SampleCode/SampleEnclave/enclave.signed.so /opt/intel/sgx-sample-app/enclave.signed.so

COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/SampleCode/QuoteGenerationSample/app /opt/intel/sgx-quote-generation/sgx-quote-generation
COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/SampleCode/QuoteGenerationSample/enclave.signed.so /opt/intel/sgx-quote-generation/enclave.signed.so

COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/SampleCode/QuoteVerificationSample/app /opt/intel/sgx-quote-verification/sgx-quote-verification
COPY --from=builder /opt/intel/SGXDataCenterAttestationPrimitives/SampleCode/QuoteVerificationSample/enclave.signed.so /opt/intel/sgx-quote-verification/enclave.signed.so

COPY --chmod=555 run-dcap-flow /opt/intel

ENTRYPOINT ["/opt/intel/sgx-sample-app/sgx-sample-app"]
