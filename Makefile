GOOS ?= $(shell go env GOOS)
GOPATH ?= $(shell go env GOPATH)
GOBIN ?= $(GOPATH)/bin
# GO111MODULE is enabled by default in modern Go; uncomment to force
# GO111MODULE = on

ENVTEST_K8S_VERSION = 1.31.0
ENVTEST_VERSION ?= release-0.19

CONTAINER_RUNTIME ?=

ifeq ($(CONTAINER_RUNTIME),)
# Try Docker first (better Rosetta 2 support on macOS ARM64)
CONTAINER_CMD ?= $(shell command -v docker 2>/dev/null)
ifeq ($(CONTAINER_CMD),)
CONTAINER_CMD := $(shell command -v podman 2>/dev/null)
endif
CONTAINER_RUNTIME=$(shell basename $(CONTAINER_CMD))
else
CONTAINER_CMD := $(shell command -v $(CONTAINER_RUNTIME) 2>/dev/null)
endif

# Platform for container builds, default is amd64
# To explicitly set platform:
#   make build-controller-image PLATFORM=linux/amd64
#   make build-controller-image PLATFORM=linux/arm64
PLATFORM ?= linux/amd64
ifneq ($(PLATFORM),)
PLATFORM_FLAG := --platform $(PLATFORM)
else
PLATFORM_FLAG :=
endif

# Extract architecture from PLATFORM for image tag suffix
# e.g., linux/amd64 -> amd64, linux/arm64 -> arm64
PLATFORM_ARCH ?= $(shell echo $(PLATFORM) | cut -d'/' -f2)
PLATFORM_SUFFIX := -$(PLATFORM_ARCH)

REGISTRY ?= quay.io
REGISTRY_ORG ?= kubev2v
REGISTRY_TAG ?= devel

VERSION ?= 99.0.0
NAMESPACE ?= konveyor-forklift
OPERATOR_NAME ?= forklift-operator
CHANNELS ?= development
DEFAULT_CHANNEL ?= development
CATALOG_NAMESPACE ?= konveyor-forklift
CATALOG_NAME ?= forklift-catalog
CATALOG_DISPLAY_NAME ?= Konveyor Forklift
CATALOG_PUBLISHER ?= Community

# Defaults for local development
VSPHERE_OS_MAP ?= forklift-virt-customize
OVIRT_OS_MAP ?= forklift-ovirt-osmap
VIRT_CUSTOMIZE_MAP ?= forklift-virt-customize
METRICS_PORT ?= 8888
METRICS_PORT_INVENTORY ?= 8889
INVENTORY_SERVICE_SCHEME ?= http

# Use OPM_OPTS="--use-http" when using a non HTTPS registry
# Use OPM_OPTS="--skip-tls-verify" when using an HTTPS registry with self-signed certificate
OPM_OPTS ?=

# By default use the controller gen installed by the
# 'controller-gen' target
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

# By default use the kubectl installed by the
# 'kubectl' target
DEFAULT_KUBECTL = $(GOBIN)/kubectl
KUBECTL ?= $(DEFAULT_KUBECTL)

# By default use the kustomize installed by the
# 'kustomize' target
DEFAULT_KUSTOMIZE = $(GOBIN)/kustomize
KUSTOMIZE ?= $(DEFAULT_KUSTOMIZE)

# Local bin directory for tools
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)


# By default use the envtest installed by the
# 'envtest' target
ENVTEST ?= $(LOCALBIN)/setup-envtest

# Image URLs to use all building/pushing image targets
# Each build image target overrides the variable of that image
# This is used for the bundle build so we don't need to build all images
# So the bundle will point to latest available images and to those which were built.
# Example: REGISTRY_ORG=mnecas0 make build-controller-image build-operator-bundle-image
# This will build controller and bundle pointing to that controller

### Components
CONTROLLER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-controller:$(REGISTRY_TAG)
API_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-api:$(REGISTRY_TAG)
VALIDATION_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-validation:$(REGISTRY_TAG)
VIRT_V2V_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-virt-v2v:$(REGISTRY_TAG)
OPERATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator:$(REGISTRY_TAG)
POPULATOR_CONTROLLER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/populator-controller:$(REGISTRY_TAG)
OVIRT_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/ovirt-populator:$(REGISTRY_TAG)
OPENSTACK_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/openstack-populator:$(REGISTRY_TAG)
OVA_PROVIDER_SERVER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-ova-provider-server:$(REGISTRY_TAG)
OVA_PROXY_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-ova-proxy:$(REGISTRY_TAG)
CLI_DOWNLOAD_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-cli-download:$(REGISTRY_TAG)
VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/vsphere-xcopy-volume-populator:$(REGISTRY_TAG)

### OLM
OPERATOR_BUNDLE_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-bundle:$(REGISTRY_TAG)
OPERATOR_INDEX_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-index:$(REGISTRY_TAG)

### External images
# These are built in separate repositories but referenced by the bundle
MUST_GATHER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-must-gather:$(REGISTRY_TAG)
UI_PLUGIN_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-console-plugin:$(REGISTRY_TAG)

# Golangci-lint version
GOLANGCI_LINT_VERSION ?= v1.64.2
GOLANGCI_LINT_BIN ?= $(GOBIN)/golangci-lint

##@ Main Targets

.PHONY: ci
ci: all tidy vendor generate-verify lint validate-forklift-controller-crd

.PHONY: all
all: test forklift-controller

##@ Testing

# Download setup-envtest locally if necessary.
.PHONY: envtest
envtest: $(ENVTEST) 
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

# Run tests
test: generate fmt vet manifests validation-test
	go test -coverprofile=cover.out ./pkg/... ./cmd/...

# Experimental e2e target
e2e-sanity: e2e-sanity-ovirt e2e-sanity-vsphere

e2e-sanity-ovirt:
	# oVirt suite
	KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go test ./tests/suit -v -ginkgo.focus ".*oVirt.*|.*Forklift.*"

e2e-sanity-vsphere:
	# vSphere suite
	go test ./tests/suit -v -ginkgo.focus ".*vSphere.*"

e2e-sanity-openstack:
	# OpenStack suite
	go test ./tests/suit -v -ginkgo.focus ".*Migration tests for OpenStack.*"

e2e-sanity-openstack-extended:
	# OpenStack extended suite
	sudo bash -c 'grep -qE "^[[:space:]]*127\.0\.0\.1[[:space:]]+packstack\.konveyor-forklift(\s|$$)" /etc/hosts || echo "127.0.0.1 packstack.konveyor-forklift" >> /etc/hosts'
	go test ./tests/suit -v -ginkgo.focus ".*Migration Extended tests for OpenStack.*" -ginkgo.parallel.total 1

e2e-sanity-ova:
	# OVA suite
	go test ./tests/suit -v -ginkgo.focus ".*OVA.*"

.PHONY: validation-test
validation-test: opa-bin
	ENVIRONMENT=test ${OPA} test validation/policies --explain fails

.PHONY: validate-forklift-controller-crd
validate-forklift-controller-crd:
	@echo "Validating ForkliftController CRD..."
	python3 hack/validate_forklift_controller_crd.py

.PHONY: integration-test
integration-test: generate fmt vet manifests envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -i --bin-dir $(LOCALBIN) -p path)" go test ./pkg/controller/migration/... -coverprofile cover.out

##@ Build & Development

# Build forklift-controller binary
forklift-controller: generate fmt vet
	go build -o bin/forklift-controller github.com/kubev2v/forklift/cmd/forklift-controller

# Ensure temporary directories for forklift services
.PHONY: ensure-temp-dirs
ensure-temp-dirs:
	install -d -m 700 /tmp/forklift-controller
	install -d -m 700 /tmp/forklift-inventory

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet ensure-temp-dirs
	VSPHERE_OS_MAP=$(VSPHERE_OS_MAP) \
	OVIRT_OS_MAP=$(OVIRT_OS_MAP) \
	VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE) \
	VIRT_CUSTOMIZE_MAP=$(VIRT_CUSTOMIZE_MAP) \
	METRICS_PORT=$(METRICS_PORT) \
	AUTH_REQUIRED=false \
	INVENTORY_SERVICE_SCHEME=$(INVENTORY_SERVICE_SCHEME) \
	WORKING_DIR=/tmp/forklift-controller \
		KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go run ./cmd/forklift-controller/main.go

# Run inventory service against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run-inventory
run-inventory: generate fmt vet ensure-temp-dirs
	VSPHERE_OS_MAP=$(VSPHERE_OS_MAP) \
	OVIRT_OS_MAP=$(OVIRT_OS_MAP) \
	VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE) \
	VIRT_CUSTOMIZE_MAP=$(VIRT_CUSTOMIZE_MAP) \
	METRICS_PORT=$(METRICS_PORT_INVENTORY) \
	ROLE=inventory \
	AUTH_REQUIRED=false \
	INVENTORY_SERVICE_SCHEME=$(INVENTORY_SERVICE_SCHEME) \
	WORKING_DIR=/tmp/forklift-inventory \
		KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go run ./cmd/forklift-controller/main.go

##@ Code Generation & Quality

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Run go mod tidy against code
tidy:
	go mod tidy

# Run go mod vendor against code
vendor:
	go mod vendor

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="./hack/boilerplate.go.txt" paths="./pkg/apis/..."

generate-verify: generate
	./hack/verify-generate.sh

##@ Kubernetes Manifests

# Generate manifests e.g. CRD, Webhooks
.PHONY: manifests
manifests: controller-gen
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role webhook paths="./pkg/apis/..." output:dir=operator/config/crd/bases

.PHONY: kustomized-manifests
kustomized-manifests: kubectl
	$(KUBECTL) kustomize operator/config/manifests > operator/.kustomized_manifests

.PHONY: generate-manifests
generate-manifests: kubectl manifests
	$(KUBECTL) kustomize operator/streams/upstream > operator/streams/upstream/upstream_manifests
	$(KUBECTL) kustomize operator/streams/downstream > operator/streams/downstream/downstream_manifests
	STREAM=upstream bash operator/streams/prepare-vars.sh
	STREAM=downstream bash operator/streams/prepare-vars.sh

.PHONY: update-manifests
update-manifests: kustomized-manifests generate-manifests
	@echo "All manifests updated successfully!"
	@echo "  - OLM bundle: operator/.kustomized_manifests"
	@echo "  - Upstream deployment: operator/streams/upstream/upstream_manifests"
	@echo "  - Downstream deployment: operator/streams/downstream/downstream_manifests"

# Install CRDs into a cluster
.PHONY: install
install: manifests kubectl
	$(KUBECTL) apply -k operator/config/crd

##@ Container Images

build-controller-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(CONTROLLER_IMAGE)$(PLATFORM_SUFFIX) -f build/forklift-controller/Containerfile .

push-controller-image: build-controller-image
	$(CONTAINER_CMD) push $(CONTROLLER_IMAGE)$(PLATFORM_SUFFIX)

build-api-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(API_IMAGE)$(PLATFORM_SUFFIX) -f build/forklift-api/Containerfile .

push-api-image: build-api-image
	$(CONTAINER_CMD) push $(API_IMAGE)$(PLATFORM_SUFFIX)

build-validation-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) --build-arg TARGETARCH=$(PLATFORM_ARCH) -t $(VALIDATION_IMAGE)$(PLATFORM_SUFFIX) -f build/validation/Containerfile .

push-validation-image: build-validation-image
	$(CONTAINER_CMD) push $(VALIDATION_IMAGE)$(PLATFORM_SUFFIX)

build-operator-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(OPERATOR_IMAGE)$(PLATFORM_SUFFIX) -f build/forklift-operator/Containerfile .

push-operator-image: build-operator-image
	$(CONTAINER_CMD) push $(OPERATOR_IMAGE)$(PLATFORM_SUFFIX)

build-virt-v2v-image: check_container_runtime
	# virt-v2v dependencies (libguestfs, nbdkit, etc.) are AMD64-only
	@if [ "$(PLATFORM_ARCH)" != "amd64" ]; then \
		echo "Notice: virt-v2v image build is only supported on amd64 platform."; \
		echo "Current platform: $(PLATFORM) - skipping virt-v2v image build."; \
	else \
		$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(VIRT_V2V_IMAGE)$(PLATFORM_SUFFIX) -f build/virt-v2v/Containerfile-upstream .; \
	fi

push-virt-v2v-image: build-virt-v2v-image
	@if [ "$(PLATFORM_ARCH)" != "amd64" ]; then \
		echo "Notice: virt-v2v image push is only supported on amd64 platform."; \
		echo "Current platform: $(PLATFORM) - skipping virt-v2v image push."; \
	else \
		$(CONTAINER_CMD) push $(VIRT_V2V_IMAGE)$(PLATFORM_SUFFIX); \
	fi

build-operator-bundle-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) \
		-t $(OPERATOR_BUNDLE_IMAGE)$(PLATFORM_SUFFIX) \
		-f build/forklift-operator-bundle/Containerfile . \
		--build-arg STREAM=dev \
		--build-arg VERSION=$(VERSION) \
		--build-arg CONTROLLER_IMAGE=$(CONTROLLER_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg API_IMAGE=$(API_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg VALIDATION_IMAGE=$(VALIDATION_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg OPERATOR_IMAGE=$(OPERATOR_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg POPULATOR_CONTROLLER_IMAGE=$(POPULATOR_CONTROLLER_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg OVIRT_POPULATOR_IMAGE=$(OVIRT_POPULATOR_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg OPENSTACK_POPULATOR_IMAGE=$(OPENSTACK_POPULATOR_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg MUST_GATHER_IMAGE=$(MUST_GATHER_IMAGE) \
		--build-arg UI_PLUGIN_IMAGE=$(UI_PLUGIN_IMAGE) \
		--build-arg CLI_DOWNLOAD_IMAGE=$(CLI_DOWNLOAD_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg OVA_PROVIDER_SERVER_IMAGE=$(OVA_PROVIDER_SERVER_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg OVA_PROXY_IMAGE=$(OVA_PROXY_IMAGE)$(PLATFORM_SUFFIX)

push-operator-bundle-image: build-operator-bundle-image
	$(CONTAINER_CMD) push $(OPERATOR_BUNDLE_IMAGE)$(PLATFORM_SUFFIX)

build-operator-index-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(OPERATOR_INDEX_IMAGE)$(PLATFORM_SUFFIX) -f build/forklift-operator-index/Containerfile . \
		--build-arg VERSION=$(VERSION) \
		--build-arg OPERATOR_BUNDLE_IMAGE=$(OPERATOR_BUNDLE_IMAGE)$(PLATFORM_SUFFIX) \
		--build-arg CHANNELS=$(CHANNELS) \
		--build-arg DEFAULT_CHANNEL=$(DEFAULT_CHANNEL) \
		--build-arg OPM_OPTS=$(OPM_OPTS)

push-operator-index-image: build-operator-index-image
	$(CONTAINER_CMD) push $(OPERATOR_INDEX_IMAGE)$(PLATFORM_SUFFIX)

build-operator-bundle-image-multiarch: check_container_runtime
	$(MAKE) build-operator-bundle-image PLATFORM_SUFFIX=

push-operator-bundle-image-multiarch: build-operator-bundle-image-multiarch
	$(CONTAINER_CMD) push $(OPERATOR_BUNDLE_IMAGE)

build-operator-index-image-multiarch: check_container_runtime
	$(MAKE) build-operator-index-image PLATFORM_SUFFIX=

push-operator-index-image-multiarch: build-operator-index-image-multiarch
	$(CONTAINER_CMD) push $(OPERATOR_INDEX_IMAGE)

build-populator-controller-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(POPULATOR_CONTROLLER_IMAGE)$(PLATFORM_SUFFIX) -f build/populator-controller/Containerfile .

push-populator-controller-image: build-populator-controller-image
	$(CONTAINER_CMD) push $(POPULATOR_CONTROLLER_IMAGE)$(PLATFORM_SUFFIX)

build-ovirt-populator-image: check_container_runtime
	# ovirt-populator dependencies (python3-ovirt-engine-sdk4, ovirt-imageio-client) are AMD64-only
	@if [ "$(PLATFORM_ARCH)" != "amd64" ]; then \
		echo "Notice: ovirt-populator image build is only supported on amd64 platform."; \
		echo "Current platform: $(PLATFORM) - skipping ovirt-populator image build."; \
	else \
		$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(OVIRT_POPULATOR_IMAGE)$(PLATFORM_SUFFIX) -f build/ovirt-populator/Containerfile-upstream .; \
	fi

push-ovirt-populator-image: build-ovirt-populator-image
	@if [ "$(PLATFORM_ARCH)" != "amd64" ]; then \
		echo "Notice: ovirt-populator image push is only supported on amd64 platform."; \
		echo "Current platform: $(PLATFORM) - skipping ovirt-populator image push."; \
	else \
		$(CONTAINER_CMD) push $(OVIRT_POPULATOR_IMAGE)$(PLATFORM_SUFFIX); \
	fi

build-openstack-populator-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(OPENSTACK_POPULATOR_IMAGE)$(PLATFORM_SUFFIX) -f build/openstack-populator/Containerfile .

push-openstack-populator-image: build-openstack-populator-image
	$(CONTAINER_CMD) push $(OPENSTACK_POPULATOR_IMAGE)$(PLATFORM_SUFFIX)

build-vsphere-xcopy-volume-populator-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE)$(PLATFORM_SUFFIX) -f build/vsphere-xcopy-volume-populator/Containerfile .

push-vsphere-xcopy-volume-populator-image: build-vsphere-xcopy-volume-populator-image
	$(CONTAINER_CMD) push $(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE)$(PLATFORM_SUFFIX)

build-ova-provider-server-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(OVA_PROVIDER_SERVER_IMAGE)$(PLATFORM_SUFFIX) -f build/ova-provider-server/Containerfile .

push-ova-provider-server-image: build-ova-provider-server-image
	$(CONTAINER_CMD) push $(OVA_PROVIDER_SERVER_IMAGE)$(PLATFORM_SUFFIX)

build-cli-download-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(CLI_DOWNLOAD_IMAGE)$(PLATFORM_SUFFIX) -f build/forklift-cli-download/Containerfile .

push-cli-download-image: build-cli-download-image
	$(CONTAINER_CMD) push $(CLI_DOWNLOAD_IMAGE)$(PLATFORM_SUFFIX)

build-ova-proxy-image: check_container_runtime
	$(CONTAINER_CMD) build $(PLATFORM_FLAG) -t $(OVA_PROXY_IMAGE)$(PLATFORM_SUFFIX) -f build/ova-proxy/Containerfile .

push-ova-proxy-image: build-ova-proxy-image
	$(CONTAINER_CMD) push $(OVA_PROXY_IMAGE)$(PLATFORM_SUFFIX)

build-all-images: build-api-image \
                  build-controller-image \
                  build-validation-image \
                  build-operator-image \
                  build-virt-v2v-image \
                  build-populator-controller-image \
                  build-ovirt-populator-image \
                  build-openstack-populator-image\
                  build-vsphere-xcopy-volume-populator-image\
                  build-ova-provider-server-image \
                  build-cli-download-image \
                  build-ova-proxy-image \
                  build-operator-bundle-image \
                  build-operator-index-image

push-all-images:  push-api-image \
                  push-controller-image \
                  push-validation-image \
                  push-operator-image \
                  push-virt-v2v-image \
                  push-populator-controller-image \
                  push-ovirt-populator-image \
                  push-openstack-populator-image\
                  push-vsphere-xcopy-volume-populator-image\
                  push-ova-provider-server-image \
                  push-cli-download-image \
                  push-ova-proxy-image \
                  push-operator-bundle-image \
				  push-operator-index-image            

##@ Multi-Architecture Manifests

push-controller-image-manifest:
	$(CONTAINER_CMD) manifest rm $(CONTROLLER_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(CONTROLLER_IMAGE) \
		$(CONTROLLER_IMAGE)-amd64 \
		$(CONTROLLER_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(CONTROLLER_IMAGE)

push-api-image-manifest:
	$(CONTAINER_CMD) manifest rm $(API_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(API_IMAGE) \
		$(API_IMAGE)-amd64 \
		$(API_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(API_IMAGE)

push-validation-image-manifest:
	$(CONTAINER_CMD) manifest rm $(VALIDATION_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(VALIDATION_IMAGE) \
		$(VALIDATION_IMAGE)-amd64 \
		$(VALIDATION_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(VALIDATION_IMAGE)

push-operator-image-manifest:
	$(CONTAINER_CMD) manifest rm $(OPERATOR_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(OPERATOR_IMAGE) \
		$(OPERATOR_IMAGE)-amd64 \
		$(OPERATOR_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(OPERATOR_IMAGE)

push-virt-v2v-image-manifest:
	$(CONTAINER_CMD) manifest rm $(VIRT_V2V_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(VIRT_V2V_IMAGE) \
		$(VIRT_V2V_IMAGE)-amd64
	$(CONTAINER_CMD) manifest push $(VIRT_V2V_IMAGE)

push-populator-controller-image-manifest:
	$(CONTAINER_CMD) manifest rm $(POPULATOR_CONTROLLER_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(POPULATOR_CONTROLLER_IMAGE) \
		$(POPULATOR_CONTROLLER_IMAGE)-amd64 \
		$(POPULATOR_CONTROLLER_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(POPULATOR_CONTROLLER_IMAGE)

push-ovirt-populator-image-manifest:
	$(CONTAINER_CMD) manifest rm $(OVIRT_POPULATOR_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(OVIRT_POPULATOR_IMAGE) \
		$(OVIRT_POPULATOR_IMAGE)-amd64
	$(CONTAINER_CMD) manifest push $(OVIRT_POPULATOR_IMAGE)

push-openstack-populator-image-manifest:
	$(CONTAINER_CMD) manifest rm $(OPENSTACK_POPULATOR_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(OPENSTACK_POPULATOR_IMAGE) \
		$(OPENSTACK_POPULATOR_IMAGE)-amd64 \
		$(OPENSTACK_POPULATOR_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(OPENSTACK_POPULATOR_IMAGE)

push-vsphere-xcopy-volume-populator-image-manifest:
	$(CONTAINER_CMD) manifest rm $(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE) \
		$(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE)-amd64 \
		$(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE)

push-ova-provider-server-image-manifest:
	$(CONTAINER_CMD) manifest rm $(OVA_PROVIDER_SERVER_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(OVA_PROVIDER_SERVER_IMAGE) \
		$(OVA_PROVIDER_SERVER_IMAGE)-amd64 \
		$(OVA_PROVIDER_SERVER_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(OVA_PROVIDER_SERVER_IMAGE)

push-cli-download-image-manifest:
	$(CONTAINER_CMD) manifest rm $(CLI_DOWNLOAD_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(CLI_DOWNLOAD_IMAGE) \
		$(CLI_DOWNLOAD_IMAGE)-amd64 \
		$(CLI_DOWNLOAD_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(CLI_DOWNLOAD_IMAGE)

push-ova-proxy-image-manifest:
	$(CONTAINER_CMD) manifest rm $(OVA_PROXY_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(OVA_PROXY_IMAGE) \
		$(OVA_PROXY_IMAGE)-amd64 \
		$(OVA_PROXY_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(OVA_PROXY_IMAGE)

push-operator-bundle-image-manifest:
	$(CONTAINER_CMD) manifest rm $(OPERATOR_BUNDLE_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(OPERATOR_BUNDLE_IMAGE) \
		$(OPERATOR_BUNDLE_IMAGE)-amd64 \
		$(OPERATOR_BUNDLE_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(OPERATOR_BUNDLE_IMAGE)

push-operator-index-image-manifest:
	$(CONTAINER_CMD) manifest rm $(OPERATOR_INDEX_IMAGE) || true
	$(CONTAINER_CMD) manifest create $(OPERATOR_INDEX_IMAGE) \
		$(OPERATOR_INDEX_IMAGE)-amd64 \
		$(OPERATOR_INDEX_IMAGE)-arm64
	$(CONTAINER_CMD) manifest push $(OPERATOR_INDEX_IMAGE)

push-all-images-manifest: push-controller-image-manifest \
                          push-api-image-manifest \
                          push-validation-image-manifest \
                          push-operator-image-manifest \
                          push-virt-v2v-image-manifest \
                          push-populator-controller-image-manifest \
                          push-ovirt-populator-image-manifest \
                          push-openstack-populator-image-manifest \
                          push-vsphere-xcopy-volume-populator-image-manifest \
                          push-ova-provider-server-image-manifest \
                          push-cli-download-image-manifest \
                          push-ova-proxy-image-manifest \
                          push-operator-bundle-image-manifest \
                          push-operator-index-image-manifest

.PHONY: check_container_runtime
check_container_runtime:
	@if [ ! -x "$(CONTAINER_CMD)" ]; then \
			echo "Container runtime was not automatically detected"; \
			echo "Please install podman or docker and make sure it's available in PATH"; \
			exit 1; \
	fi

##@ Deployment

.PHONY: deploy-operator-index
deploy-operator-index: kubectl
	export OPERATOR_INDEX_IMAGE=${OPERATOR_INDEX_IMAGE}$(PLATFORM_SUFFIX); envsubst < operator/forklift-operator-catalog.yaml | $(KUBECTL) apply -f -

.PHONY: deploy-operator-index-multiarch
deploy-operator-index-multiarch: kubectl
	export OPERATOR_INDEX_IMAGE=${OPERATOR_INDEX_IMAGE}; envsubst < operator/forklift-operator-catalog.yaml | $(KUBECTL) apply -f -

.PHONY: setup-k8s-prerequisites
setup-k8s-prerequisites: ## Install prerequisites for Forklift on k8s (cert-manager, CDI, CNA, KubeVirt, OLM). Usage: make setup-k8s-prerequisites
	./hack/setup-k8s-prerequisites.sh

.PHONY: deploy-k8s
deploy-k8s: kubectl ## Deploy Forklift on k8s with forklift-k8s-dev.yaml using local images. Usage: make deploy-k8s [REGISTRY=quay.io] [REGISTRY_ORG=kubev2v] [REGISTRY_TAG=devel] [PLATFORM=linux/amd64]
	export OPERATOR_INDEX_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-index:$(REGISTRY_TAG)$(PLATFORM_SUFFIX); envsubst < operator/forklift-k8s-dev.yaml | $(KUBECTL) apply -f -

.PHONY: deploy-k8s-multiarch
deploy-k8s-multiarch: kubectl ## Deploy Forklift on k8s with forklift-k8s-dev.yaml using local images (multiarch). Usage: make deploy-k8s-multiarch [REGISTRY=quay.io] [REGISTRY_ORG=kubev2v] [REGISTRY_TAG=devel]
	export OPERATOR_INDEX_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-index:$(REGISTRY_TAG); envsubst < operator/forklift-k8s-dev.yaml | $(KUBECTL) apply -f -

.PHONY: deploy-k8s-controller
deploy-k8s-controller: kubectl ## Deploy ForkliftController CR on k8s. Usage: make deploy-k8s-controller [NAMESPACE=konveyor-forklift]
	@NAMESPACE=${NAMESPACE}; \
	if [ -z "$$NAMESPACE" ]; then \
		NAMESPACE=konveyor-forklift; \
	fi; \
	KUBECTL=$(KUBECTL) ./hack/deploy-k8s-controller.sh $$NAMESPACE

##@ Tool Installation

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.3

.PHONY: kubectl
kubectl: $(KUBECTL)
$(DEFAULT_KUBECTL):
	curl -L https://dl.k8s.io/release/v1.25.10/bin/linux/amd64/kubectl -o $(GOBIN)/kubectl && chmod +x $(GOBIN)/kubectl

.PHONY: kustomize
kustomize: $(KUSTOMIZE)
$(DEFAULT_KUSTOMIZE):
	go install sigs.k8s.io/kustomize/kustomize/v5@v5.7.0

mockgen-install:
	go install go.uber.org/mock/mockgen@v0.4.0

opa-bin:
ifeq (, $(shell command -v opa))
	GOBIN=$(GOBIN) go install github.com/open-policy-agent/opa@v1.8.0
OPA=$(GOBIN)/opa
else
OPA=$(shell which opa)
endif

ROOTLESS ?= true

define DEPLOYMENT_VARS
OPERATOR_NAMESPACE=$(NAMESPACE)
OPERATOR_NAME=$(OPERATOR_NAME)
SUBSCRIPTION_CHANNEL=$(CHANNELS)
CATALOG_NAMESPACE=$(CATALOG_NAMESPACE)
CATALOG_NAME=$(CATALOG_NAME)
CATALOG_DISPLAY_NAME=$(CATALOG_DISPLAY_NAME)
CATALOG_IMAGE=$(OPERATOR_INDEX_IMAGE)
CATALOG_PUBLISHER=$(CATALOG_PUBLISHER)
REGISTRY_ORG=$(REGISTRY_ORG)
endef
export DEPLOYMENT_VARS

##@ Code Quality

.PHONY: lint-install
lint-install:
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	GOBIN=$(GOBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "golangci-lint installed successfully."

.PHONY: lint
lint: $(GOLANGCI_LINT_BIN)
	@echo "Running golangci-lint..."
	$(GOLANGCI_LINT_BIN) run --timeout 10m ./pkg/... ./cmd/...

.PHONY: update-tekton
update-tekton:
	SKIP_UPDATE=false ./update-tekton.sh .tekton/*.yaml

.PHONY: validate-commits validate-commits-range
validate-commits:
	@echo "Validating commit messages..."
	@./scripts/validate-commits.sh --verbose

validate-commits-range:
	@echo "Validating commit messages in range: $(RANGE)"
	@./scripts/validate-commits.sh --range "$(RANGE)" --verbose

$(GOLANGCI_LINT_BIN):
	$(MAKE) lint-install


