CONTROLLER_GEN ?= controller-gen
GO := go
GOFMT := gofmt
KUSTOMIZE ?= kustomize
OPERATOR_SDK ?= operator-sdk

BUILDTAGS ?= ""
BUILDER ?= "docker"
EXTRA_BUILD_ARGS ?= ""

CERT_MANAGER_VERSION ?= v1.15.2
CONTROLLER_GEN_VERSION ?= v0.18.0
GOLANGCI_LINT_VERSION ?= v2.4.0
KIND_VERSION ?= v0.23.0
GOLICENSES_VERSION ?= v1.6.0
# Default bundle image tag
BUNDLE_IMG ?= intel-device-plugins-controller-bundle:$(TAG)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)
OLM_MANIFESTS = deployments/operator/manifests
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(TAG) $(BUNDLE_METADATA_OPTS) --kustomize-dir $(OLM_MANIFESTS) --output-dir . --use-image-digests
BUNDLE_DIR = community-operators/operators/intel-device-plugins-operator/$(TAG)

TESTDATA_DIR = pkg/topology/testdata

EXTRA_BUILD_ARGS += --build-arg GOLICENSES_VERSION=$(GOLICENSES_VERSION)

pkgs  = $(shell $(GO) list ./... | grep -v vendor | grep -v e2e | grep -v envtest)
cmds = $(shell ls --ignore=internal cmd)

all: build

format:
	@report=`$(GOFMT) -s -d -w $$(find cmd pkg test -name \*.go)` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

vendor:
	@$(GO) mod vendor -v

install-tools:
	GO111MODULE=on $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GO) install sigs.k8s.io/kind@${KIND_VERSION}

go-mod-tidy:
	$(GO) mod download all
	@report=`$(GO) mod tidy -v 2>&1` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

update-fixture:
	@scripts/ttar -C $(TESTDATA_DIR) -c -f $(TESTDATA_DIR)/sys.ttar sys/

fixture:
	@scripts/ttar --recursive-unlink -C $(TESTDATA_DIR) -x -f $(TESTDATA_DIR)/sys.ttar

test: fixture
ifndef WHAT
	@$(GO) test -tags $(BUILDTAGS) -race -coverprofile=coverage.txt -covermode=atomic $(pkgs)
else
	@cd $(WHAT) && \
	    $(GO) test -tags $(BUILDTAGS) -v -race -cover -coverprofile cover.out || rc=1; \
	    $(GO) tool cover -html=cover.out -o coverage.html; \
	    rm cover.out; \
	    echo "Coverage report: file://$$(realpath coverage.html)"; \
	    exit $$rc
endif

test-with-kind: fixture intel-sgx-admissionwebhook intel-fpga-admissionwebhook intel-deviceplugin-operator install-tools
	# Build a Cluster with KinD & Load Images & Install Cert-Manager
	kind create cluster
	kind load docker-image $(REG)intel-sgx-admissionwebhook:$(TAG)
	kind load docker-image $(REG)intel-fpga-admissionwebhook:$(TAG)
	kind load docker-image $(REG)intel-deviceplugin-operator:$(TAG)
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml
	# Test SGX Admission Webhook, FPGA Admission Webhook and Device Plugin Operator Manager's Webhook
	$(GO) test -v ./test/e2e -args -kubeconfig ~/.kube/config -ginkgo.focus "SGX Admission"
	$(GO) test -v ./test/e2e -args -kubeconfig ~/.kube/config -ginkgo.focus "FPGA Admission"
	$(GO) test -v ./test/e2e -args -kubeconfig ~/.kube/config -ginkgo.focus "Operator"

envtest:
	@$(GO) test ./test/envtest

lint:
	@golangci-lint run --timeout 15m

checks: lint go-mod-tidy

generate:
	$(CONTROLLER_GEN) object:headerFile="build/boilerplate/boilerplate.go.txt" paths="./pkg/apis/..."
	$(CONTROLLER_GEN) crd:crdVersions=v1 \
		paths="./pkg/apis/..." \
		output:crd:artifacts:config=deployments/operator/crd/bases
	$(CONTROLLER_GEN) crd:crdVersions=v1 \
		paths="./pkg/apis/fpga/..." \
		output:crd:artifacts:config=deployments/fpga_admissionwebhook/crd/bases
	$(CONTROLLER_GEN) webhook \
		paths="./pkg/..." \
		output:webhook:artifacts:config=deployments/operator/webhook
	$(CONTROLLER_GEN) webhook \
		paths="./pkg/fpgacontroller/..." \
		output:webhook:artifacts:config=deployments/fpga_admissionwebhook/webhook
	$(CONTROLLER_GEN) webhook \
		paths="./pkg/webhooks/sgx/..." \
		output:webhook:artifacts:config=deployments/sgx_admissionwebhook/webhook
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/..." output:dir=deployments/operator/rbac
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/fpgacontroller/..." output:dir=deployments/fpga_admissionwebhook/rbac

$(cmds):
	cd cmd/$@; $(GO) build -tags $(BUILDTAGS)

build: $(cmds)

.PHONY: bundle
bundle:
	rm -rf $(BUNDLE_DIR)
	mkdir -p $(BUNDLE_DIR)
	$(OPERATOR_SDK) generate kustomize manifests -q --input-dir $(OLM_MANIFESTS) --output-dir $(OLM_MANIFESTS) --apis-dir pkg/apis
	$(KUSTOMIZE) build $(OLM_MANIFESTS) | sed "s|intel-deviceplugin-operator:devel|intel-deviceplugin-operator:$(TAG)|" | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	# Remove unneeded resources
	rm manifests/*service.yaml
	rm manifests/*clusterrole.yaml
	# Put generated files in a directory
	mv manifests metadata bundle.Dockerfile $(BUNDLE_DIR)
	$(OPERATOR_SDK) bundle validate $(BUNDLE_DIR)/

bundle-build:
	$(BUILDER) build -f $(BUNDLE_DIR)/bundle.Dockerfile -t $(BUNDLE_IMG) $(BUNDLE_DIR)

clean:
	@for cmd in $(cmds) ; do pwd=$(shell pwd) ; cd cmd/$$cmd ; $(GO) clean ; cd $$pwd ; done

ORG?=intel
REG?=$(ORG)/
TAG?=devel
export TAG

ifeq ($(E2E_LEVEL), $(filter $(E2E_LEVEL), full))
  GENERATED_SKIP_OPT=-ginkgo.skip "App:noapp"
else ifeq ($(E2E_LEVEL),basic)
  ADDITIONAL_FOCUS_REGEX=App:noapp
else
  $(error Unsupported E2E_LEVEL value: $(E2E_LEVEL). Possible options: full, basic)
endif
GENERATED_SKIP_OPT += $(if $(SKIP),-ginkgo.skip "$(SKIP)")
ADDITIONAL_FOCUS_REGEX := $(if $(FOCUS),$(FOCUS).*$(ADDITIONAL_FOCUS_REGEX),$(ADDITIONAL_FOCUS_REGEX))

e2e-qat:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.show-node-events -ginkgo.focus "Device:qat.*$(ADDITIONAL_FOCUS_REGEX)" $(GENERATED_SKIP_OPT) -delete-namespace-on-failure=false

e2e-sgx:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.show-node-events -ginkgo.focus "Device:sgx.*$(ADDITIONAL_FOCUS_REGEX)" $(GENERATED_SKIP_OPT) -delete-namespace-on-failure=false

e2e-gpu:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.show-node-events -ginkgo.focus "Device:gpu.*$(ADDITIONAL_FOCUS_REGEX)" $(GENERATED_SKIP_OPT) -delete-namespace-on-failure=false

e2e-dsa:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.show-node-events -ginkgo.focus "Device:dsa.*$(ADDITIONAL_FOCUS_REGEX)" $(GENERATED_SKIP_OPT) -delete-namespace-on-failure=false

e2e-iaa:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.show-node-events -ginkgo.focus "Device:iaa.*$(ADDITIONAL_FOCUS_REGEX)" $(GENERATED_SKIP_OPT) -delete-namespace-on-failure=false

e2e-spr:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.show-node-events -ginkgo.focus "Device:(iaa|dsa)|Device:qat.*Mode:dpdk.*Resource:(cy|dc).*" -ginkgo.focus "Device:sgx.*|(SGX Admission)" -ginkgo.focus "Device:gpu.*Resource:i915" $(GENERATED_SKIP_OPT) -delete-namespace-on-failure=false

pre-pull:
ifeq ($(TAG),devel)
	@$(BUILDER) pull golang:1.25-trixie
	@$(BUILDER) pull debian:unstable-slim
	@$(BUILDER) pull ubuntu:22.04
endif

dockerlib = build/docker/lib
dockertemplates = build/docker/templates
images = $(shell basename -s .Dockerfile.in -a $(dockertemplates)/*.Dockerfile.in | grep -v -e dlb -e fpga -e xpumanager-sidecar)
dockerfiles = $(shell basename -s .in -a $(dockertemplates)/*.Dockerfile.in | xargs -I"{}" echo build/docker/{})

test-image-base-layer:
	@scripts/test-image-base-layer.sh $(IMG) $(BUILDER)

$(dockerfiles): $(dockertemplates)/*.Dockerfile.in $(dockerlib)/*.docker
	@cat $(dockerlib)/default_header.docker $(dockertemplates)/$(shell basename $@.in) \
	| cpp -w -nostdinc -I./$(dockerlib) -DINPUT_FILENAME=$(dockertemplates)/$(shell basename $@.in) -C -P \
	| sed 's/\\N/\\/' > $@

clean-dockerfiles:
	@rm -f build/docker/*.Dockerfile

dockerfiles: clean-dockerfiles $(dockerfiles)

check-dockerfiles: dockerfiles
	@GITDIFF=`git diff build/docker` ; \
	if [ -n "$$GITDIFF" ]; then \
		echo "Changes detected in Dockerfiles:" && \
		echo "$$GITDIFF" && \
		echo "Please commit result of \"make dockerfiles\" to make them in synch with the .in -files." ; \
		exit 1; \
	fi

$(images): $(dockerfiles)
	@build/docker/build-image.sh $(REG)$@ $(BUILDER) $(EXTRA_BUILD_ARGS)

images: $(images)

demos = $(shell basename -a demo/*/ | grep -v -e dlb -e opae-nlb-demo)

$(demos):
	@cd demo/ && ./build-image.sh $(REG)$@ $(BUILDER)

demos: $(demos)

set-version:
	@scripts/set-version.sh $(TAG)

null  :=
space := $(null) #
comma := ,
images_json := $(subst $(space),$(comma),[$(addprefix ",$(addsuffix ",$(images) $(demos))]))

check-github-actions:
	@python3 -c 'import sys, yaml, json; json.dump(yaml.load(sys.stdin, Loader=yaml.SafeLoader), sys.stdout)' < .github/workflows/lib-build.yaml | \
	jq -e '$(images_json) - .jobs.image.strategy.matrix.image == []' > /dev/null || \
	(echo "Make sure all images are listed in .github/workflows/lib-build.yaml"; exit 1)

.PHONY: all format test lint build images $(cmds) $(images) lock-images vendor pre-pull set-version check-github-actions envtest fixture update-fixture install-tools test-image-base-layer

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
	. ${@D}/bin/activate && pip install wheel && pip install -r $<
	touch $@

clean-licenses:
	rm -rf licenses

licenses = $(shell ls --ignore=internal cmd | xargs -I"{}" echo licenses/{})

$(licenses):
	@rm -rf $@
	@license_cmd=`basename $@` && echo fetching licenses for $$license_cmd && \
	GO111MODULE=on go run github.com/google/go-licenses@$(GOLICENSES_VERSION) save "./cmd/$$license_cmd" --save_path licenses/$$license_cmd

.PHONY: clean-licenses licenses $(licenses)

licenses: $(licenses)
