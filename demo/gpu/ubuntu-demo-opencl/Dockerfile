FROM ubuntu:devel

RUN apt-get update && \
    apt-get install -y locales git apt-transport-https && \
    apt-get upgrade -y && \
    apt-get install -y gawk wget git-core diffstat unzip texinfo gcc-multilib \
        build-essential cmake pkg-config python ocl-icd-dev libegl1-mesa-dev \
	ocl-icd-opencl-dev libdrm-dev libxfixes-dev libxext-dev \
	llvm-3.8-dev clang-3.8 libclang-3.8-dev libtinfo-dev libedit-dev \
	zlib1g-dev clinfo strace && \
    sed -i -e 's/# en_US.UTF-8 UTF-8/en_US.UTF-8 UTF-8/' /etc/locale.gen && \
    echo 'LANG="en_US.UTF-8"'>/etc/default/locale && \
    dpkg-reconfigure --frontend=noninteractive locales && \
    update-locale LANG=en_US.UTF-8

ENV LANG en_US.UTF-8

COPY *_beignet_tests.sh /
RUN mkdir /workspace && /build_beignet_tests.sh /workspace
