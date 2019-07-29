FROM clearlinux:base as builder

ARG QAT_DRIVER_RELEASE="qat1.7.l.4.6.0-00025"

RUN swupd bundle-add wget c-basic go-basic && \
    mkdir -p /usr/src/qat && \
    cd /usr/src/qat && \
    wget https://01.org/sites/default/files/downloads/$QAT_DRIVER_RELEASE.tar.gz && \
    tar xf *.tar.gz
RUN cd /usr/src/qat/quickassist/utilities/adf_ctl && \
    make KERNEL_SOURCE_DIR=/usr/src/qat/quickassist/qat && \
    cp -a adf_ctl /usr/bin/
ARG DIR=/go/src/github.com/intel/intel-device-plugins-for-kubernetes
WORKDIR $DIR
COPY . .
RUN cd cmd/qat_plugin; go install
RUN chmod a+x /go/bin/qat_plugin

FROM gcr.io/distroless/cc
COPY --from=builder /go/bin/qat_plugin /usr/bin/intel_qat_device_plugin
COPY --from=builder /usr/bin/adf_ctl /usr/bin/adf_ctl
CMD ["/usr/bin/intel_qat_device_plugin"]
