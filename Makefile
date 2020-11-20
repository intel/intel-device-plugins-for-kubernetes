CONTROLLER_GEN ?= controller-gen
GO := go
GOFMT := gofmt
KUBECTL ?= kubectl
KIND ?= kind
PODMAN ?= podman

BUILDTAGS ?= ""
BUILDER ?= "docker"
EXTRA_BUILD_ARGS ?= ""

WEBHOOK_IMAGE_FILE = intel-fpga-admissionwebhook-devel.tgz

pkgs  = $(shell $(GO) list ./... | grep -v vendor | grep -v e2e | grep -v envtest)
cmds = $(shell ls cmd)
e2e_tmp_dir := $(shell mktemp -u -t e2e-tests.XXXXXXXXXX)

all: build

format:
	@report=`$(GOFMT) -s -d -w $$(find cmd pkg test -name \*.go)` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

vendor:
	@$(GO) mod vendor -v

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
	@$(PODMAN) tag localhost/intel/intel-fpga-admissionwebhook:0.19.0 docker.io/intel/intel-fpga-admissionwebhook:0.19.0
	@mkdir -p $(e2e_tmp_dir)
	@$(PODMAN) save "docker.io/intel/intel-fpga-admissionwebhook:0.19.0" -o $(e2e_tmp_dir)/$(WEBHOOK_IMAGE_FILE)
	@$(KIND) create cluster --name "intel-device-plugins" --kubeconfig $(e2e_tmp_dir)/kubeconfig --image "kindest/node:v1.19.0"
	@$(KIND) load image-archive --name "intel-device-plugins" $(e2e_tmp_dir)/$(WEBHOOK_IMAGE_FILE)
	$(KUBECTL) --kubeconfig=$(e2e_tmp_dir)/kubeconfig apply -f https://github.com/jetstack/cert-manager/releases/download/v1.0.3/cert-manager.yaml
	@$(GO) test -v ./test/e2e -args -kubeconfig $(e2e_tmp_dir)/kubeconfig -kubectl-path $(KUBECTL) -ginkgo.focus "Webhook" || rc=1; \
	$(KIND) delete cluster --name "intel-device-plugins"; \
	rm -rf $(e2e_tmp_dir); \
	exit $$rc

envtest:
	@$(GO) test ./test/envtest

lint:
	@golangci-lint run --timeout 15m

checks: lint go-mod-tidy

generate:
	$(CONTROLLER_GEN) object:headerFile="build/boilerplate/boilerplate.go.txt" paths="./pkg/apis/..."
	$(CONTROLLER_GEN) crd:crdVersions=v1beta1,trivialVersions=true \
		paths="./pkg/apis/..." \
		output:crd:artifacts:config=deployments/operator/crd/bases
	$(CONTROLLER_GEN) crd:crdVersions=v1beta1,trivialVersions=true \
		paths="./pkg/apis/fpga.intel.com/..." \
		output:crd:artifacts:config=deployments/fpga_admissionwebhook/crd/bases
	$(CONTROLLER_GEN) webhook \
		paths="./pkg/..." \
		output:webhook:artifacts:config=deployments/operator/webhook
	$(CONTROLLER_GEN) webhook \
		paths="./pkg/fpgacontroller/..." \
		output:webhook:artifacts:config=deployments/fpga_admissionwebhook/webhook
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/..." output:dir=deployments/operator/rbac
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/fpgacontroller/..." output:dir=deployments/fpga_admissionwebhook/rbac

$(cmds):
	cd cmd/$@; $(GO) build -tags $(BUILDTAGS)

build: $(cmds)

deploy-operator: operator generate
	kubectl apply -k deployments/operator/default

undeploy-operator:
	kubectl delete -k deployments/operator/default

run-operator: deploy-operator
	./cmd/operator/operator

clean:
	@for cmd in $(cmds) ; do pwd=$(shell pwd) ; cd cmd/$$cmd ; $(GO) clean ; cd $$pwd ; done

ORG?=intel
REG?=$(ORG)/
TAG?=0.19.0
export TAG

pre-pull:
ifeq ($(TAG),devel)
	@$(BUILDER) pull clearlinux/golang:latest
	@$(BUILDER) pull clearlinux:latest
endif

images = $(shell ls build/docker/*.Dockerfile | sed 's/.*\/\(.\+\)\.Dockerfile/\1/')

$(images):
	@build/docker/build-image.sh $(REG)$@ $(BUILDER) $(EXTRA_BUILD_ARGS)

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

null  :=
space := $(null) #
comma := ,
images_json := $(subst $(space),$(comma),[$(addprefix ",$(addsuffix ",$(images) $(demos))]))

check-github-actions:
	@python3 -c 'import sys, yaml, json; json.dump(yaml.load(sys.stdin), sys.stdout)' < .github/workflows/ci.yaml | \
	jq -e '$(images_json) - .jobs.image.strategy.matrix.image == []' > /dev/null || \
	(echo "Make sure all images are listed in .github/workflows/ci.yaml"; exit 1)

.PHONY: all format test lint build images $(cmds) $(images) lock-images vendor pre-pull set-version check-github-actions run-operator envtest deploy-operator undeploy-operator

SPHINXOPTS    =
SPHINXBUILD   = sphinx-build
SOURCEDIR     = .
BUILDDIR      = _build

# Generate doc site under _build/html with Sphinx.
vhtml: _work/venv/.stamp
	. _work/venv/bin/activate && \
		$(SPHINXBUILD) -M html "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)
		cp docs/index.html $(BUILDDIR)/html/index.html

html:
		$(SPHINXBUILD) -M html "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)
		cp docs/index.html $(BUILDDIR)/html/index.html

clean-html:
	rm -rf $(BUILDDIR)/html

# Set up a Python3 environment with the necessary tools for document creation.
_work/venv/.stamp: docs/requirements.txt
	rm -rf ${@D}
	python3 -m venv ${@D}
	. ${@D}/bin/activate && pip install -r $<
	touch $@