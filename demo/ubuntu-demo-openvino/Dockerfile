FROM ubuntu:18.04 as builder
ARG DOWNLOAD_LINK=http://registrationcenter-download.intel.com/akdlm/irc_nas/16345/l_openvino_toolkit_p_2020.1.023.tgz
ARG INSTALL_DIR=/opt/intel/openvino
ARG TEMP_DIR=/tmp/openvino_installer
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    cpio \
    sudo \
    python3-pip \
    python3-setuptools \
    libboost-filesystem1.65 \
    libboost-thread1.65 \
    lsb-release
RUN mkdir -p $TEMP_DIR; curl -o $TEMP_DIR/openvino.tgz $DOWNLOAD_LINK
RUN cd $TEMP_DIR && \
    tar xf openvino.tgz && \
    cd l_openvino_toolkit* && \
    sed -i 's/decline/accept/g' silent.cfg && \
    ./install.sh -s silent.cfg && \
    rm -rf $TEMP_DIR
RUN $INSTALL_DIR/install_dependencies/install_openvino_dependencies.sh
# build Inference Engine samples
RUN mkdir $INSTALL_DIR/deployment_tools/inference_engine/samples/cpp/build && cd $INSTALL_DIR/deployment_tools/inference_engine/samples/cpp/build && \
    /bin/bash -c "source $INSTALL_DIR/bin/setupvars.sh && cmake .. && make -j1"
RUN pip3 install networkx==2.3
RUN cd $INSTALL_DIR/deployment_tools/demo && \
    /bin/bash -c "source $INSTALL_DIR/bin/setupvars.sh && ./demo_squeezenet_download_convert_run.sh"
RUN  cp /opt/intel/openvino/deployment_tools/demo/car.png /root && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/lib/intel64/plugins.xml /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/lib/intel64/libHDDLPlugin.so /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/external/hddl/lib/libhddlapi.so /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/external/hddl/lib/libion.so.0 /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/external/hddl/lib/libmvnc-hddl.so.0 /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/external/hddl/lib/libbsl.so.0 /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /lib/x86_64-linux-gnu/libusb-1.0.so.0 /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp -r /opt/intel/openvino/deployment_tools/inference_engine/external/hddl /root && \
     /bin/bash -c "source /opt/intel/openvino/bin/setupvars.sh && \
     ldd /root/inference_engine_samples_build/intel64/Release/classification_sample_async" | grep opt | awk '{print $3}' | xargs -Iaaa cp aaa /root/inference_engine_samples_build/intel64/Release/lib/

FROM ubuntu:18.04
RUN apt-get update && apt-get install -y --no-install-recommends \
    libjson-c3 \
    libboost-filesystem1.65 \
    libboost-thread1.65 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY do_classification.sh /
COPY --from=builder /root/ /root/
