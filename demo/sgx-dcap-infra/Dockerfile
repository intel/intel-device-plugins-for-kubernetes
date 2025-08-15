FROM ubuntu:24.04

# TODO: pin DCAP release version

RUN apt update && apt install -y curl gnupg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/intel-sgx.gpg] https://download.01.org/intel-sgx/sgx_repo/ubuntu noble main" | \
    tee -a /etc/apt/sources.list.d/intel-sgx.list \
    && curl -s https://download.01.org/intel-sgx/sgx_repo/ubuntu/intel-sgx-deb.key | \
    gpg --dearmor --output /usr/share/keyrings/intel-sgx.gpg \
    && apt update \
    && apt install -y --no-install-recommends \
       tdx-qgs \
       sgx-pck-id-retrieval-tool \
       libsgx-ra-uefi \
       libsgx-dcap-default-qpl

# BUG: "qgs -p=0" gets overriden by the config file making the parameter useless
RUN sed -e 's/\(^port =\).*/\1 0/g' -i /etc/qgs.conf

COPY dcap-registration-flow /usr/bin

ENTRYPOINT ["/opt/intel/tdx-qgs/qgs", "--no-daemon", "-p=0"]
