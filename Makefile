CONTROLLER_GEN ?= controller-gen
GO := go
GOFMT := gofmt
KUSTOMIZE ?= kustomize
OPERATOR_SDK ?= operator-sdk

BUILDTAGS ?= ""
BUILDER ?= "docker"
EXTRA_BUILD_ARGS ?= ""

CHECKOV_HIGH_SEVERITY ?= CKV_K8S_2,CKV_K8S_16,CKV_K8S_39,CKV_K8S_100,CKV_K8S_102,CKV_K8S_104,CKV_K8S_105,CKV_K8S_108,CKV_K8S_110,CKV_K8S_111,CKV_K8S_113,CKV_K8S_115,CKV_K8S_116,CKV_K8S_118,CKV_K8S_119,CKV_K8S_121,CKV_K8S_148,CKV_K8S_149,CKV_K8S_71,CKV_K8S_72,CKV_K8S_73,CKV_K8S_86,CKV_K8S_88,CKV_K8S_96,CKV_K8S_99
CHECKOV_MEDIUM_SEVERITY ?= CKV_K8S_1,CKV_K8S_3,CKV_K8S_4,CKV_K8S_6,CKV_K8S_17,CKV_K8S_18,CKV_K8S_19,CKV_K8S_20,CKV_K8S_23,CKV_K8S_27,CKV_K8S_5,CKV_K8S_106,CKV_K8S_107,CKV_K8S_112,CKV_K8S_117,CKV_K8S_138,CKV_K8S_150,CKV_K8S_74,CKV_K8S_75,CKV_K8S_78,CKV_K8S_79,CKV_K8S_80,CKV_K8S_85,CKV_K8S_91,CKV_K8S_95,CKV_K8S_97,CKV_K8S_49,CKV_K8S_6,CKV_K8S_79
CHECKOV_SOFT_FAIL_ON ?= CKV_K8S_16,CKV_K8S_19,CKV_K8S_39,CKV_K8S_20,CKV_K8S_23
CERT_MANAGER_VERSION ?= v1.6.1
CONTROLLER_GEN_VERSION ?= v0.8.0
GOLANGCI_LINT_VERSION ?= v1.43.0
KIND_VERSION ?= v0.11.1
# Current Operator version
OPERATOR_VERSION ?= 0.23.0
# Previous Operator version
OPERATOR_PREVIOUS_VERSION ?= 0.21.0
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
PACKAGEMANIFESTS_DIR = community-operators/operators/intel-device-plugins-operator

TESTDATA_DIR = pkg/topology/testdata

pkgs  = $(shell $(GO) list ./... | grep -v vendor | grep -v e2e | grep -v envtest)
cmds = $(shell ls --ignore=internal cmd)

all: build

format:
	@report=`$(GOFMT) -s -d -w $$(find cmd pkg test -name \*.go)` ; if [ -n "$$report" ]; then echo "$$report"; exit 1; fi

vendor:
	@$(GO) mod vendor -v

install-tools:
	GO111MODULE=on $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GO) install sigs.k8s.io/kind@${KIND_VERSION}
	
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

test-with-kind: fixture intel-sgx-admissionwebhook intel-fpga-admissionwebhook intel-deviceplugin-operator install-tools
	# Build a Cluster with KinD & Load Images & Install Cert-Manager
	kind create cluster
	kind load docker-image $(REG)intel-sgx-admissionwebhook:$(TAG)
	kind load docker-image $(REG)intel-fpga-admissionwebhook:$(TAG)
	kind load docker-image $(REG)intel-deviceplugin-operator:$(TAG)
	kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml
	# Test SGX Admission Webhook & FPGA Admission Webhook
	$(GO) test -v ./test/e2e -args -kubeconfig ~/.kube/config -ginkgo.focus "SGX Admission"
	$(GO) test -v ./test/e2e -args -kubeconfig ~/.kube/config -ginkgo.focus "FPGA Admission"
	# Deploy Operator
	kubectl apply -k deployments/operator/default/

envtest:
	@$(GO) test ./test/envtest

lint:
	@golangci-lint run --timeout 15m

checkov: _work/venv/.stamp
	_work/venv/bin/pip3 install -U checkov
	_work/venv/bin/checkov \
		-c ${CHECKOV_HIGH_SEVERITY},${CHECKOV_MEDIUM_SEVERITY} \
		--quiet --compact \
		--soft-fail-on ${CHECKOV_SOFT_FAIL_ON} \
		--framework kustomize \
		--directory ./deployments --skip-path "(apps?|aesmd)$$"
	
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
	$(KUSTOMIZE) build $(OLM_MANIFESTS) | sed "s|intel-deviceplugin-operator:devel|intel-deviceplugin-operator:$(OPERATOR_VERSION)|" | $(OPERATOR_SDK) generate packagemanifests -q --kustomize-dir $(OLM_MANIFESTS) --version $(OPERATOR_VERSION) --from-version $(OPERATOR_PREVIOUS_VERSION) $(BUNDLE_METADATA_OPTS) --output-dir $(PACKAGEMANIFESTS_DIR)
	# Remove unneeded resources
	rm $(PACKAGEMANIFESTS_DIR)/$(OPERATOR_VERSION)/*service.yaml
	rm $(PACKAGEMANIFESTS_DIR)/$(OPERATOR_VERSION)/*clusterrole.yaml

clean:
	@for cmd in $(cmds) ; do pwd=$(shell pwd) ; cd cmd/$$cmd ; $(GO) clean ; cd $$pwd ; done

ORG?=intel
REG?=$(ORG)/
TAG?=devel
export TAG

e2e-fpga:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.progress -ginkgo.focus "FPGA" -delete-namespace-on-failure=false

e2e-qat:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.progress -ginkgo.focus "QAT plugin in DPDK mode" -delete-namespace-on-failure=false

e2e-sgx:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.progress -ginkgo.focus "SGX" -delete-namespace-on-failure=false

e2e-gpu:
	@$(GO) test -v ./test/e2e/... -ginkgo.v -ginkgo.progress -ginkgo.focus "GPU" -delete-namespace-on-failure=false

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

.PHONY: all format test lint build images $(cmds) $(images) lock-images vendor pre-pull set-version check-github-actions envtest fixture update-fixture install-tools

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
