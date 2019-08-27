GO := go
GOFMT := gofmt
GOCYCLO := gocyclo

BUILDTAGS ?= ""
BUILDER ?= "docker"

pkgs  = $(shell $(GO) list ./... | grep -v vendor)
cmds = $(shell ls cmd)

all: build

format:
	@report=`$(GOFMT) -s -d -w $$(find cmd pkg -name \*.go)` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

vet:
	@$(GO) vet -v -vettool=$$(which shadow) $(pkgs)

cyclomatic-check:
	@report=`$(GOCYCLO) -over 15 cmd pkg`; if [ -n "$$report" ]; then echo "Complexity is over 15 in"; echo $$report; exit 1; fi

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

lint:
	@rc=0 ; for f in $$(find -name \*.go | grep -v \.\/vendor) ; do golint -set_exit_status $$f || rc=1 ; done ; exit $$rc

$(cmds):
	cd cmd/$@; $(GO) build -tags $(BUILDTAGS)

build: $(cmds)

clean:
	@for cmd in $(cmds) ; do pwd=$(shell pwd) ; cd cmd/$$cmd ; $(GO) clean ; cd $$pwd ; done

ORG?=intel
REG?=$(ORG)/
TAG?=devel
export TAG

images = $(shell ls build/docker/*.Dockerfile | grep -v clearlinux-base | sed 's/.*\/\(.\+\)\.Dockerfile/\1/')

clearlinux-base:
	@test -d mod -o ! -d $(shell $(GO) env GOPATH)/pkg/mod && (echo "Either no cache or not clean workspace detected. Make sure there's no 'mod' directory in the repo and Go mod cache is present. Exiting..."; exit 1) || true
	@mv $(shell $(GO) env GOPATH)/pkg/mod .
	@rc=0; build/docker/build-image.sh $@ $(BUILDER) || rc=1; mv mod $(shell $(GO) env GOPATH)/pkg/; exit $$rc

ifeq ($(USE_CACHED_GO_MODULES),yes)
$(images): clearlinux-base
	@build/docker/build-image.sh --build-arg CLEAR_LINUX_BASE=clearlinux-base:devel --no-pull $(REG)$@ $(BUILDER)
else
$(images):
	@build/docker/build-image.sh $(REG)$@ $(BUILDER)
endif

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

.PHONY: all format vet cyclomatic-check test lint build images $(cmds) $(images) lock-images $(images_tags) push
