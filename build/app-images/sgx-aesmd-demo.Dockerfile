# This Dockerfile is currently provided as a reference to build aesmd with ECDSA attestation
# but is not published along with the device plugin container images.
FROM ubuntu:20.04

RUN apt update && apt install -y curl gnupg \
    && echo 'deb [arch=amd64] https://download.01.org/intel-sgx/sgx_repo/ubuntu focal main' | tee /etc/apt/sources.list.d/intel-sgx.list \
    && curl -s https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | apt-key add - \
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
