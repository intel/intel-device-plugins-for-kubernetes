FROM ubuntu:16.04

# install OPAE build tools and dependencies
RUN apt-get update && apt-get install -y cmake gcc g++ patch wget zip libboost-all-dev libjson-c-dev uuid-dev

COPY hello_fpga.diff /opt/build/

# download OPAE sources
RUN mkdir -p /opt/build && \
    cd /opt/build && \
    wget https://github.com/OPAE/opae-sdk/archive/0.13.0.tar.gz -O- | tar -zx

# patch, build and install OPAE and hello_fpga
RUN cd /opt/build/opae-sdk-0.13.0 && \
    patch -p0 < ../hello_fpga.diff && \
    mkdir mybuild && \
    cd mybuild && \
    cmake .. -DBUILD_ASE=1 && \
    make install && \
    cp ./bin/hello_fpga /usr/bin/

COPY run_hello_fpga.sh /usr/bin/
