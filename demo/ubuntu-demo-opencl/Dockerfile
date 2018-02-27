FROM ubuntu:bionic

COPY run-opencl-example.sh /
COPY fft.patch /
COPY expected.pgm /

RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y locales git ocl-icd-opencl-dev build-essential \
                       clinfo jq uprightdiff wget unzip && \
    sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    echo 'LANG="en_US.UTF-8"'>/etc/default/locale && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8 && \
    cd /root && wget https://github.com/intel/compute-runtime/releases/download/18.26.10987/intel-opencl_18.26.10987_amd64.deb && \
    dpkg -i /root/*.deb && \
    ldconfig && \
    wget https://us.fixstars.com/files/opencl/samples.zip && \
    unzip samples.zip && \
    cd 6-1/fft && \
    patch -p0 < /fft.patch && \
    gcc -g fft.cpp -o fft -lOpenCL -lm

ENV LANG en_US.UTF-8
