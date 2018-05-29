GO := go
pkgs  = $(shell $(GO) list ./... | grep -v vendor)

DOCKER_ARGS = --build-arg HTTP_PROXY --build-arg HTTPS_PROXY --build-arg NO_PROXY --build-arg http_proxy --build-arg https_proxy --build-arg no_proxy --pull
GPU_IMAGE = intel-gpu-plugin
FPGA_IMAGE = intel-fpga-plugin

all: build

format:
	@$(GO) fmt $(pkgs)

vet:
	@$(GO) vet -v -shadow $(pkgs)

test:
	@$(GO) test -cover $(pkgs)

lint:
	@rc=0 ; for f in $$(find -name \*.go | grep -v \.\/vendor) ; do golint -set_exit_status $$f || rc=1 ; done ; exit $$rc

TAG?=$(shell git rev-parse HEAD)

build:
	cd cmd/gpu_plugin; go build
	cd cmd/fpga_plugin; go build

container:
	docker build -f gpu.Dockerfile  $(DOCKER_ARGS) -t ${GPU_IMAGE}:${TAG} .
	docker build -f fpga.Dockerfile $(DOCKER_ARGS) -t ${FPGA_IMAGE}:${TAG} .

.PHONY: all format vet build container
