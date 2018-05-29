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
	@$(GO) test -race -coverprofile=coverage.txt -covermode=atomic $(pkgs)

lint:
	@rc=0 ; for f in $$(find -name \*.go | grep -v \.\/vendor) ; do golint -set_exit_status $$f || rc=1 ; done ; exit $$rc

TAG?=$(shell git rev-parse HEAD)

build:
	cd plugins/gpu_plugin; go build
	cd plugins/fpga_plugin; go build

container:
	docker build -f docker/gpu.Dockerfile  $(DOCKER_ARGS) -t ${GPU_IMAGE}:${TAG} .
	docker build -f docker/fpga.Dockerfile $(DOCKER_ARGS) -t ${FPGA_IMAGE}:${TAG} .

gpu_plugin:
	cd plugins/gpu_plugin; go build
	docker build -f docker/gpu.Dockerfile  $(DOCKER_ARGS) -t ${GPU_IMAGE}:${TAG} .

fpga_plugin:
	cd plugins/fpga_plugin; go build
	docker build -f docker/fpga.Dockerfile $(DOCKER_ARGS) -t ${FPGA_IMAGE}:${TAG} .


.PHONY: all format vet build container
