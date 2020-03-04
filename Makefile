GO := go
GOFMT := gofmt
GOCYCLO := gocyclo
KUBECTL ?= kubectl
KIND ?= kind
PODMAN ?= podman

BUILDTAGS ?= ""
BUILDER ?= "docker"

WEBHOOK_IMAGE_FILE = intel-fpga-admissionwebhook-devel.tgz

pkgs  = $(shell $(GO) list ./... | grep -v vendor | grep -v e2e)
cmds = $(shell ls cmd)
e2e_tmp_dir := $(shell mktemp -u -t e2e-tests.XXXXXXXXXX)

all: build

format:
	@report=`$(GOFMT) -s -d -w $$(find cmd pkg test -name \*.go)` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

vet:
	@$(GO) vet -v -vettool=$$(which shadow) $(pkgs)

vendor:
	@$(GO) mod vendor -v

cyclomatic-check:
	@report=`$(GOCYCLO) -over 15 cmd pkg test`; if [ -n "$$report" ]; then echo "Complexity is over 15 in"; echo $$report; exit 1; fi

go-mod-tidy:
	$(GO) mod download
	@report=`$(GO) mod tidy -v 2>&1` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

test:
ifndef WHAT
	@$(GO) test -tags $(BUILDTAGS) -race -coverprofile=coverage.txt -covermode=atomic $(pkgs)
else
	@cd $(WHAT) && \
            $(GO) test -tags $(BUILDTAGS) -v -cover -coverprofile cover.out || rc=1; \
            $(GO) tool cover -html=cover.out -o coverage.html; \
            rm cover.out; \
            echo "Coverage report: file://$$(realpath coverage.html)"; \
            exit $$rc
endif

test-with-kind:
	@build/docker/build-image.sh intel/intel-fpga-admissionwebhook buildah
	@$(PODMAN) tag localhost/intel/intel-fpga-admissionwebhook:0.17.0 docker.io/intel/intel-fpga-admissionwebhook:0.17.0
	@mkdir -p $(e2e_tmp_dir)
	@$(PODMAN) save "docker.io/intel/intel-fpga-admissionwebhook:0.17.0 -o $(e2e_tmp_dir)/$(WEBHOOK_IMAGE_FILE)
	@$(KIND) create cluster --name "intel-device-plugins" --kubeconfig $(e2e_tmp_dir)/kubeconfig --image "kindest/node:v1.17.0"
	@$(KIND) load image-archive --name "intel-device-plugins" $(e2e_tmp_dir)/$(WEBHOOK_IMAGE_FILE)
	@$(GO) test -v ./test/e2e -args -kubeconfig $(e2e_tmp_dir)/kubeconfig -kubectl-path $(KUBECTL) -ginkgo.focus "Webhook" || rc=1; \
	$(KIND) delete cluster --name "intel-device-plugins"; \
	rm -rf $(e2e_tmp_dir); \
	exit $$rc

lint:
	@rc=0 ; for f in $$(find -name \*.go | grep -v \.\/vendor) ; do golint -set_exit_status $$f || rc=1 ; done ; exit $$rc

checks: lint format cyclomatic-check go-mod-tidy

$(cmds):
	cd cmd/$@; $(GO) build -tags $(BUILDTAGS)

build: $(cmds)

clean:
	@for cmd in $(cmds) ; do pwd=$(shell pwd) ; cd cmd/$$cmd ; $(GO) clean ; cd $$pwd ; done

ORG?=intel
REG?=$(ORG)/
TAG?=0.17.0
export TAG

pre-pull:
ifeq ($(TAG),devel)
	@$(BUILDER) pull clearlinux/golang:latest
	@$(BUILDER) pull clearlinux:latest
endif

images = $(shell ls build/docker/*.Dockerfile | sed 's/.*\/\(.\+\)\.Dockerfile/\1/')

$(images):
	@build/docker/build-image.sh $(REG)$@ $(BUILDER)

images: $(images)

demos = $(shell cd demo/ && ls -d */ | sed 's/\(.\+\)\//\1/g')

$(demos):
	@cd demo/ && ./build-image.sh $(REG)$@ $(BUILDER)

demos: $(demos)

image_tags = $(patsubst %,$(REG)%,$(images) $(demos))
$(image_tags):
	@docker push $@

push: $(image_tags)

lock-images:
	@scripts/update-clear-linux-base.sh clearlinux/golang:latest $(shell ls build/docker/*.Dockerfile)
	@scripts/update-clear-linux-base.sh clearlinux:latest $(shell find demo -name Dockerfile)

set-version:
	@scripts/set-version.sh $(TAG)

.PHONY: all format vet cyclomatic-check test lint build images $(cmds) $(images) lock-images vendor pre-pull set-version
