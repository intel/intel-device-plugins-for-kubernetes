FROM ubuntu:16.04

# install OPAE build tools and dependencies
RUN apt-get update && apt-get install -y wget libjson0 uuid-dev

ENV OPAE_URL https://github.com/OPAE/opae-sdk/releases/download/1.0.0-5

# download OPAE sources
RUN mkdir -p /opt/build && \
    cd /opt/build && \
    wget $OPAE_URL/opae-libs-1.0.0.x86_64.deb $OPAE_URL/opae-tools-1.0.0.x86_64.deb \
         $OPAE_URL/opae-tools-extra-1.0.0.x86_64.deb $OPAE_URL/opae-devel-1.0.0.x86_64.deb && \
    dpkg -i *.deb

COPY test_fpga.sh /usr/bin/
