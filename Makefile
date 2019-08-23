GO := go
GOFMT := gofmt
GOCYCLO := gocyclo

BUILDTAGS ?= ""

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

images = $(shell ls build/docker/*.Dockerfile | sed 's/.*\/\(.\+\)\.Dockerfile/\1/')

$(images):
	@build/docker/build-image.sh $(REG)$@ $(BUILDER)

images: $(images)

demos = $(shell cd demo/ && ls -d */ | sed 's/\(.\+\)\//\1/g')

$(demos):
	@cd demo/ && ./build-image.sh $(REG)$@ $(BUILDER)

demos: $(demos)

lock-images:
	@scripts/update-clear-linux-base.sh clearlinux/golang:latest $(shell ls build/docker/*.Dockerfile)
	@scripts/update-clear-linux-base.sh clearlinux:latest $(shell find demo -name Dockerfile)

.PHONY: all format vet cyclomatic-check test lint build images $(cmds) $(images) lock-images
