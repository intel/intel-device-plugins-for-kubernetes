GO := go
GOFMT := gofmt
GOCYCLO := gocyclo

pkgs  = $(shell $(GO) list ./... | grep -v vendor)
cmds = $(shell ls cmd)

all: build

format:
	@report=`$(GOFMT) -s -d -w $$(find cmd internal -name \*.go)` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

vet:
	@$(GO) vet -v -shadow $(pkgs)

cyclomatic-check:
	@report=`$(GOCYCLO) -over 15 cmd internal`; if [ -n "$$report" ]; then echo "Complexity is over 15 in"; echo $$report; exit 1; fi

test:
ifndef WHAT
	@$(GO) test -race -coverprofile=coverage.txt -covermode=atomic $(pkgs)
else
	@cd $(WHAT) && \
            $(GO) test -v -cover -coverprofile cover.out || rc=1; \
            $(GO) tool cover -html=cover.out -o coverage.html; \
            rm cover.out; \
            echo "Coverage report: file://$$(realpath coverage.html)"; \
            exit $$rc
endif

lint:
	@rc=0 ; for f in $$(find -name \*.go | grep -v \.\/vendor) ; do golint -set_exit_status $$f || rc=1 ; done ; exit $$rc

$(cmds):
	cd cmd/$@; go build

build: $(cmds)

clean:
	@for cmd in $(cmds) ; do pwd=$(shell pwd) ; cd cmd/$$cmd ; go clean ; cd $$pwd ; done

TAG?=$(shell git rev-parse HEAD)

images = $(shell ls build/docker/*.Dockerfile | sed 's/.*\/\(.\+\)\.Dockerfile/\1/')

$(images):
	docker build -f build/docker/$@.Dockerfile --pull -t $@:$(TAG) .
	docker tag $@:$(TAG) $@:devel

images: $(images)

demos = $(shell cd demo/ && ls -d */ | sed 's/\(.\+\)\//\1/g' | grep -v crypto-perf)

$(demos):
	@cd demo/ && ./build-image.sh $@

demos: $(demos)

.PHONY: all format vet cyclomatic-check test lint build images $(cmds) $(images)
