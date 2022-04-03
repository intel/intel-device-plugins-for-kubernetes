FROM ubuntu:18.04 as builder
ARG INSTALL_DIR=/opt/intel/openvino
ARG VERSION=2020.2.130
RUN apt update
RUN apt install -y gnupg2 curl sudo
RUN curl  https://apt.repos.intel.com/openvino/2020/GPG-PUB-KEY-INTEL-OPENVINO-2020 | apt-key add -
RUN echo 'deb https://apt.repos.intel.com/openvino/2020 all main' > /etc/apt/sources.list.d/intel-openvino.list
RUN apt update  
RUN apt install -y --no-install-recommends \
	intel-openvino-ie-rt-hddl-ubuntu-bionic-$VERSION \
	intel-openvino-ie-samples-$VERSION \
	intel-openvino-setupvars-$VERSION \
	intel-openvino-omz-dev-$VERSION \
	intel-openvino-omz-tools-$VERSION \
	intel-openvino-model-optimizer-$VERSION \
	intel-openvino-ie-rt-cpu-ubuntu-bionic-$VERSION \
	intel-openvino-opencv-etc-$VERSION \
	intel-openvino-opencv-generic-$VERSION \
	intel-openvino-opencv-lib-ubuntu-bionic-$VERSION \
	intel-openvino-pot-$VERSION

RUN $INSTALL_DIR/install_dependencies/install_openvino_dependencies.sh
# build Inference Engine samples
RUN $INSTALL_DIR/deployment_tools/inference_engine/samples/cpp/build_samples.sh
RUN $INSTALL_DIR/deployment_tools/demo/demo_squeezenet_download_convert_run.sh
RUN  cp /opt/intel/openvino/deployment_tools/demo/car.png /root && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/lib/intel64/plugins.xml /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /opt/intel/openvino/deployment_tools/inference_engine/lib/intel64/libHDDLPlugin.so /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp /lib/x86_64-linux-gnu/libusb-1.0.so.0 /root/inference_engine_samples_build/intel64/Release/lib/ && \
     cp -r /opt/intel/openvino/deployment_tools/inference_engine/external/hddl /root && \
     /bin/bash -c "source /opt/intel/openvino/bin/setupvars.sh && \
     ldd /root/inference_engine_samples_build/intel64/Release/classification_sample_async" | grep opt | awk '{print $3}' | xargs -Iaaa cp aaa /root/inference_engine_samples_build/intel64/Release/lib/ && \
     /bin/bash -c "source /opt/intel/openvino/bin/setupvars.sh && \
     ldd /opt/intel/openvino/deployment_tools/inference_engine/lib/intel64/libHDDLPlugin.so" | grep opt | awk '{print $3}' | xargs -Iaaa cp aaa /root/inference_engine_samples_build/intel64/Release/lib/

FROM ubuntu:18.04
RUN apt-get update && apt-get install -y --no-install-recommends \
    libjson-c3 \
    libboost-filesystem1.65 \
    libboost-thread1.65 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY do_classification.sh /
COPY --from=builder /root/ /root/
