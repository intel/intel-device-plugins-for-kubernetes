FROM ubuntu:24.04 AS build

ENV DEBIAN_FRONTEND=noninteractive

RUN apt update && apt install -y build-essential git git-lfs cmake python3 && rm -rf /var/lib/apt/lists/\*

ENV NPU_RELEASE=v1.19.0
ENV NPU_PROJECT=https://github.com/intel/linux-npu-driver/

RUN mkdir /install

RUN git clone -b ${NPU_RELEASE} --recurse-submodules --depth 1 ${NPU_PROJECT} && \
  cd linux-npu-driver && \
  cmake -DCMAKE_INSTALL_PREFIX=/install/usr -DCMAKE_INSTALL_LIBDIR=lib -B build -S . && \
  cmake --build build --parallel $(nproc) && \
  cmake --install build

FROM ubuntu:24.04 AS final

COPY --from=build /install /

COPY npu-tests.sh /

ENTRYPOINT [ "/npu-tests.sh" ]
