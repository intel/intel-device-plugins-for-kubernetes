FROM gramineproject/gramine:1.7-jammy

RUN apt-get update \
 && env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    stress-ng \
    make

COPY * /stress-ng/
WORKDIR /stress-ng

RUN gramine-sgx-gen-private-key && \
    make SGX=1
ENTRYPOINT ["/usr/bin/gramine-sgx"]
