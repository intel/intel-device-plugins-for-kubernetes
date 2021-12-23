CONTROLLER_GEN ?= controller-gen
GO := go
GOFMT := gofmt
KIND ?= kind
KUBECTL ?= kubectl
KUSTOMIZE ?= kustomize
OPERATOR_SDK ?= operator-sdk
PODMAN ?= podman

BUILDTAGS ?= ""
BUILDER ?= "docker"
EXTRA_BUILD_ARGS ?= ""

# Current Operator version
OPERATOR_VERSION ?= 0.21.0
# Previous Operator version
OPERATOR_PREVIOUS_VERSION ?= 0.20.0
# Default bundle image tag
BUNDLE_IMG ?= intel-device-plugins-controller-bundle:$(OPERATOR_VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)
OLM_MANIFESTS = deployments/operator/manifests

WEBHOOK_IMAGE_FILE = intel-fpga-admissionwebhook-devel.tgz

TESTDATA_DIR = pkg/topology/testdata

pkgs  = $(shell $(GO) list ./... | grep -v vendor | grep -v e2e | grep -v envtest)
cmds = $(shell ls --ignore=internal cmd)
e2e_tmp_dir := $(shell mktemp -u -t e2e-tests.XXXXXXXXXX)

all: build

format:
	@report=`$(GOFMT) -s -d -w $$(find cmd pkg test -name \*.go)` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

vendor:
	@$(GO) mod vendor -v

go-mod-tidy:
	$(GO) mod download
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

test-with-kind: fixture
	@build/docker/build-image.sh intel/intel-fpga-admissionwebhook buildah
	@$(PODMAN) tag localhost/intel/intel-fpga-admissionwebhook:devel docker.io/intel/intel-fpga-admissionwebhook:devel
	@mkdir -p $(e2e_tmp_dir)
	@$(PODMAN) save "docker.io/intel/intel-fpga-admissionwebhook:devel" -o $(e2e_tmp_dir)/$(WEBHOOK_IMAGE_FILE)
	@$(KIND) create cluster --name "intel-device-plugins" --kubeconfig $(e2e_tmp_dir)/kubeconfig --image "kindest/node:v1.19.0"
	@$(KIND) load image-archive --name "intel-device-plugins" $(e2e_tmp_dir)/$(WEBHOOK_IMAGE_FILE)
	$(KUBECTL) --kubeconfig=$(e2e_tmp_dir)/kubeconfig apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.yaml
	@$(GO) test -v ./test/e2e -args -kubeconfig $(e2e_tmp_dir)/kubeconfig -kubectl-path $(KUBECTL) -ginkgo.focus "FPGA Admission" || rc=1; \
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
	$(CONTROLLER_GEN) rbac:roleName=gpu-manager-role paths="./cmd/gpu_plugin/..." output:dir=deployments/operator/rbac
	cp deployments/operator/rbac/role.yaml deployments/operator/rbac/gpu_manager_role.yaml
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/..." output:dir=deployments/operator/rbac
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="./pkg/fpgacontroller/..." output:dir=deployments/fpga_admissionwebhook/rbac

$(cmds):
	cd cmd/$@; $(GO) build -tags $(BUILDTAGS)

build: $(cmds)

.PHONY: bundle
bundle:
	$(OPERATOR_SDK) generate kustomize manifests -q --input-dir $(OLM_MANIFESTS) --output-dir $(OLM_MANIFESTS) --apis-dir pkg/apis
	$(KUSTOMIZE) build $(OLM_MANIFESTS) | sed "s|intel-deviceplugin-operator:devel|intel-deviceplugin-operator:$(OPERATOR_VERSION)|" | $(OPERATOR_SDK) generate bundle -q --overwrite --kustomize-dir $(OLM_MANIFESTS) --version $(OPERATOR_VERSION) $(BUNDLE_METADATA_OPTS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: packagemanifests
packagemanifests:
	$(OPERATOR_SDK) generate kustomize manifests -q --input-dir $(OLM_MANIFESTS) --output-dir $(OLM_MANIFESTS) --apis-dir pkg/apis
	$(KUSTOMIZE) build $(OLM_MANIFESTS) | sed "s|intel-deviceplugin-operator:devel|intel-deviceplugin-operator:$(OPERATOR_VERSION)|" | $(OPERATOR_SDK) generate packagemanifests -q --kustomize-dir $(OLM_MANIFESTS) --version $(OPERATOR_VERSION) --from-version $(OPERATOR_PREVIOUS_VERSION) $(BUNDLE_METADATA_OPTS)
	# Remove unneeded resources
	rm packagemanifests/$(OPERATOR_VERSION)/*service.yaml
	rm packagemanifests/$(OPERATOR_VERSION)/*clusterrole.yaml

clean:
	@for cmd in $(cmds) ; do pwd=$(shell pwd) ; cd cmd/$$cmd ; $(GO) clean ; cd $$pwd ; done

ORG?=intel
REG?=$(ORG)/
TAG?=devel
export TAG

e2e-fpga:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.progress -ginkgo.focus "FPGA Plugin" -delete-namespace-on-failure=false

e2e-qat:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.progress -ginkgo.focus "QAT plugin in DPDK mode" -delete-namespace-on-failure=false

e2e-sgx:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.progress -ginkgo.focus "SGX" -delete-namespace-on-failure=false

pre-pull:
ifeq ($(TAG),devel)
	@$(BUILDER) pull golang:1.17-bullseye
	@$(BUILDER) pull debian:unstable-slim
	@$(BUILDER) pull clearlinux:latest
endif

images = $(shell basename -s .Dockerfile -a build/docker/*.Dockerfile)

$(images):
	@build/docker/build-image.sh $(REG)$@ $(BUILDER) $(EXTRA_BUILD_ARGS)

images: $(images)

demos = $(shell basename -a demo/*/)

$(demos):
	@cd demo/ && ./build-image.sh $(REG)$@ $(BUILDER)

demos: $(demos)

image_tags = $(patsubst %,$(REG)%\:$(TAG),$(images) $(demos))
$(image_tags):
	@docker push $@

push: $(image_tags)

lock-images:
	@scripts/update-clear-linux-base.sh clearlinux:latest $(shell find demo -name Dockerfile)

set-version:
	@scripts/set-version.sh $(TAG)

null  :=
space := $(null) #
comma := ,
images_json := $(subst $(space),$(comma),[$(addprefix ",$(addsuffix ",$(images) $(demos))]))

check-github-actions:
	@python3 -c 'import sys, yaml, json; json.dump(yaml.load(sys.stdin, Loader=yaml.SafeLoader), sys.stdout)' < .github/workflows/ci.yaml | \
	jq -e '$(images_json) - .jobs.image.strategy.matrix.image == []' > /dev/null || \
	(echo "Make sure all images are listed in .github/workflows/ci.yaml"; exit 1)

.PHONY: all format test lint build images $(cmds) $(images) lock-images vendor pre-pull set-version check-github-actions envtest fixture update-fixture

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
