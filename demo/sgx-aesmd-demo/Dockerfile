# This Dockerfile is currently provided as a reference to build aesmd with ECDSA attestation
# but is not published along with the device plugin container images.
FROM ubuntu:26.04

# Pins the SGX PSW/DCAP packages to a known version
ARG SGX_APT_PREFERENCES=99sgx_2_30_100_resolute_custom_version.cfg

RUN apt update && apt install -y curl gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu resolute main" | \
    tee -a /etc/apt/sources.list.d/intel-sgx.list \
    && curl -s https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
    gpg --dearmor --output /usr/share/keyrings/intel-sgx.gpg \
    && curl -s -o /etc/apt/preferences.d/intel-sgx https://download.01.org/intel-sgx/sgx_repo/ubuntu/apt_preference_files/${SGX_APT_PREFERENCES} \
    && apt update \
    && apt install -y --no-install-recommends \
       sgx-aesm-service \
       libsgx-dcap-ql \
       libsgx-aesm-ecdsa-plugin \
       libsgx-aesm-pce-plugin \
       libsgx-aesm-quote-ex-plugin \
       libsgx-dcap-default-qpl

RUN echo "/opt/intel/sgx-aesm-service/aesm" | tee /etc/ld.so.conf.d/sgx.conf \
    && ldconfig

ENV PATH=/opt/intel/sgx-aesm-service/aesm
ENTRYPOINT ["/opt/intel/sgx-aesm-service/aesm/aesm_service", "--no-daemon"]
