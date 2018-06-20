GO := go
GOCYCLO := gocyclo

pkgs  = $(shell $(GO) list ./... | grep -v vendor)
cmds = $(shell ls cmd)

# Image names are formed after corresponding cmd names, e.g. "gpu_plugin" -> "intel-gpu-plugin"
images = $(shell echo $(cmds) | tr _ - | sed 's/[^ ]* */intel-&/g')

DOCKER_ARGS = --build-arg HTTP_PROXY --build-arg HTTPS_PROXY --build-arg NO_PROXY --build-arg http_proxy --build-arg https_proxy --build-arg no_proxy --pull

all: build

format:
	@$(GO) fmt $(pkgs)

vet:
	@$(GO) vet -v -shadow $(pkgs)

cyclomatic-check:
	@report=`$(GOCYCLO) -over 15 cmd internal`; if [ -n "$$report" ]; then echo "Complexity is over 15 in"; echo $$report; exit 1; fi

test:
ifndef WHAT
	@$(GO) test -race -coverprofile=coverage.txt -covermode=atomic $(pkgs)
else
	@cd $(WHAT) && \
            $(GO) test -v -cover -coverprofile cover.out -args -logtostderr -v 2 || rc=1; \
            $(GO) tool cover -html=cover.out -o coverage.html; \
            rm cover.out; \
            echo "Coverage report: file://$$(realpath coverage.html)"; \
            exit $$rc
endif

lint:
	@rc=0 ; for f in $$(find -name \*.go | grep -v \.\/vendor) ; do golint -set_exit_status $$f || rc=1 ; done ; exit $$rc

TAG?=$(shell git rev-parse HEAD)

$(cmds):
	cd cmd/$@; go build

build: $(cmds)

$(images):
	docker build -f build/docker/$@.Dockerfile $(DOCKER_ARGS) -t $@:$(TAG) .
	docker tag $@:$(TAG) $@:devel

images: $(images)

.PHONY: all format vet cyclomatic-check test lint build images $(cmds) $(images)
