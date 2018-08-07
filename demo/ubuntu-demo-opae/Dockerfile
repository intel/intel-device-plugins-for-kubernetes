FROM ubuntu:16.04

# install OPAE build tools and dependencies
RUN apt-get update && apt-get install -y wget libjson0 uuid-dev

ENV VERSION 1.1.0-2
ENV OPAE_URL https://github.com/OPAE/opae-sdk/releases/download/$VERSION

# download OPAE sources
RUN mkdir -p /opt/build && \
    cd /opt/build && \
    wget $OPAE_URL/opae-libs-$VERSION.x86_64.deb $OPAE_URL/opae-tools-$VERSION.x86_64.deb \
         $OPAE_URL/opae-tools-extra-$VERSION.x86_64.deb $OPAE_URL/opae-devel-$VERSION.x86_64.deb && \
    dpkg -i *.deb

COPY test_fpga.sh /usr/bin/
