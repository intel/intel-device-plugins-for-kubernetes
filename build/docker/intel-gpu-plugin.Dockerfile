FROM golang:1.11-alpine as builder
ARG DIR=/go/src/github.com/intel/intel-device-plugins-for-kubernetes
WORKDIR $DIR
COPY . .
RUN cd cmd/gpu_plugin; go install
RUN chmod a+x /go/bin/gpu_plugin

FROM alpine
COPY --from=builder /go/bin/gpu_plugin /usr/bin/intel_gpu_device_plugin
CMD ["/usr/bin/intel_gpu_device_plugin"]
