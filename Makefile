GOOS ?= `go env GOOS`
GOPATH ?= `go env GOPATH`
GOBIN ?= $(GOPATH)/bin
GO111MODULE = auto

CONTAINER_CMD ?= $(shell command -v podman)
ifeq ($(CONTAINER_CMD),)
CONTAINER_CMD := $(shell command -v docker)
endif

REGISTRY ?= quay.io
REGISTRY_ACCOUNT ?= kubev2v
REGISTRY_TAG ?= devel

VERSION ?= 2.4.1
NAMESPACE ?= konveyor-forklift
CHANNELS ?= development
DEFAULT_CHANNEL ?= development

# Use OPM_OPTS="--use-http" when using a non HTTPS registry
OPM_OPTS ?=

# By default use the controller gen installed by the
# 'controller-gen' target
DEFAULT_CONTROLLER_GEN = $(GOBIN)/controller-gen
CONTROLLER_GEN ?= $(DEFAULT_CONTROLLER_GEN)

# Image URLs to use all building/pushing image targets
CONTROLLER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-controller:$(REGISTRY_TAG)
API_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-api:$(REGISTRY_TAG)
VALIDATION_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-validation:$(REGISTRY_TAG)
VIRT_V2V_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-virt-v2v:$(REGISTRY_TAG)
VIRT_V2V_WARM_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-virt-v2v-warm:$(REGISTRY_TAG)
OPERATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-operator:$(REGISTRY_TAG)
OPERATOR_BUNDLE_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-operator-bundle:$(REGISTRY_TAG)
OPERATOR_INDEX_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/forklift-operator-index:$(REGISTRY_TAG)
POPULATOR_CONTROLLER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/populator-controller:$(REGISTRY_TAG)
OVIRT_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/ovirt-populator:$(REGISTRY_TAG)
OPENSTACK_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ACCOUNT)/openstack-populator:$(REGISTRY_TAG)

### External images
MUST_GATHER_IMAGE ?= quay.io/kubev2v/forklift-must-gather:latest
MUST_GATHER_API_IMAGE ?= quay.io/kubev2v/forklift-must-gather-api:latest
UI_IMAGE ?= quay.io/kubev2v/forklift-ui:latest
UI_PLUGIN_IMAGE ?= quay.io/kubev2v/forklift-console-plugin:latest

BAZEL_OPTS ?= --verbose_failures

ifneq (,$(findstring /usr/lib64/ccache,$(PATH)))
CCACHE_DIR ?= $${HOME}/.ccache
BAZEL_OPTS +=	--sandbox_writable_path=$(CCACHE_DIR)
$(shell [ -d $(CCACHE_DIR) ] || mkdir -p $(CCACHE_DIR))
endif

XDG_RUNTIME_DIR ?=
ifneq (,$(XDG_RUNTIME_DIR))
BAZEL_OPTS +=	--sandbox_writable_path=$${XDG_RUNTIME_DIR}
$(shell [ -d $(XDG_RUNTIME_DIR) ] || mkdir -p $(XDG_RUNTIME_DIR))
endif

ci: all

all: test forklift-controller

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Experimental e2e target
e2e-sanity: e2e-sanity-ovirt e2e-sanity-vsphere

e2e-sanity-ovirt:
	# ovirt suite
	go test ./tests/suit -v -ginkgo.focus ".*oVirt.*"

e2e-sanity-vsphere:
	# vsphere suit
	go test ./tests/suit -v -ginkgo.focus ".*vSphere.*"

e2e-sanity-openstack:
	# openstack suit
	go test ./tests/suit -v -ginkgo.focus ".*OpenStack.*"

# Build forklift-controller binary
forklift-controller: generate fmt vet
	go build -o bin/forklift-controller github.com/konveyor/forklift-controller/cmd/forklift-controller

# Build manager binary with compiler optimizations disabled
debug: generate fmt vet
	go build -o bin/forklift-controller -gcflags=all="-N -l" github.com/konveyor/forklift-controller/cmd/forklift-controller

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	export METRICS_PORT=8888;\
		KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go run ./cmd/forklift-controller/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, Webhooks
manifests: controller-gen
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role webhook paths="./pkg/apis/..." output:dir=operator/config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="./hack/boilerplate.go.txt" paths="./pkg/apis/..."

build-controller-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/forklift-controller:forklift-controller-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-controller-image: build-controller-image
	$(CONTAINER_CMD) tag bazel/cmd/forklift-controller:forklift-controller-image $(CONTROLLER_IMAGE)
	$(CONTAINER_CMD) push $(CONTROLLER_IMAGE)

build-api-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/forklift-api:forklift-api-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-api-image: build-api-image
	$(CONTAINER_CMD) tag bazel/cmd/forklift-api:forklift-api-image $(API_IMAGE)
	$(CONTAINER_CMD) push $(API_IMAGE)

build-validation-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run validation:forklift-validation-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-validation-image: build-validation-image
	$(CONTAINER_CMD) tag bazel/validation:forklift-validation-image $(VALIDATION_IMAGE)
	$(CONTAINER_CMD) push $(VALIDATION_IMAGE)

build-operator-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run operator:forklift-operator-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-operator-image: build-operator-image
	$(CONTAINER_CMD) tag bazel/operator:forklift-operator-image $(OPERATOR_IMAGE)
	$(CONTAINER_CMD) push $(OPERATOR_IMAGE)

build-virt-v2v-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run --package_path=virt-v2v/cold forklift-virt-v2v \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-virt-v2v-image: build-virt-v2v-image
	$(CONTAINER_CMD) tag bazel:forklift-virt-v2v $(VIRT_V2V_IMAGE)
	$(CONTAINER_CMD) push $(VIRT_V2V_IMAGE)

build-virt-v2v-warm-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run --package_path=virt-v2v/warm forklift-virt-v2v-warm \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-virt-v2v-warm-image: build-virt-v2v-warm-image
	$(CONTAINER_CMD) tag bazel:forklift-virt-v2v-warm ${VIRT_V2V_WARM_IMAGE}
	$(CONTAINER_CMD) push ${VIRT_V2V_WARM_IMAGE}

build-operator-bundle-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run operator:forklift-operator-bundle-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD) \
		--action_env VERSION=$(VERSION) \
		--action_env NAMESPACE=$(NAMESPACE) \
		--action_env CHANNELS=$(CHANNELS) \
		--action_env DEFAULT_CHANNEL=$(DEFAULT_CHANNEL) \
		--action_env OPERATOR_IMAGE=$(OPERATOR_IMAGE) \
		--action_env MUST_GATHER_IMAGE=$(MUST_GATHER_IMAGE) \
		--action_env MUST_GATHER_API_IMAGE=$(MUST_GATHER_API_IMAGE) \
		--action_env UI_IMAGE=$(UI_IMAGE) \
		--action_env UI_PLUGIN_IMAGE=$(UI_PLUGIN_IMAGE) \
		--action_env VALIDATION_IMAGE=$(VALIDATION_IMAGE) \
		--action_env VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE) \
		--action_env VIRT_V2V_WARM_IMAGE=$(VIRT_V2V_WARM_IMAGE) \
		--action_env CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) \
		--action_env API_IMAGE=$(API_IMAGE) \
		--action_env POPULATOR_CONTROLLER_IMAGE=$(POPULATOR_CONTROLLER_IMAGE) \
		--action_env OVIRT_POPULATOR_IMAGE=$(OVIRT_POPULATOR_IMAGE) \
		--action_env OPENSTACK_POPULATOR_IMAGE=$(OPENSTACK_POPULATOR_IMAGE)

push-operator-bundle-image: build-operator-bundle-image
	 $(CONTAINER_CMD) tag bazel/operator:forklift-operator-bundle-image $(OPERATOR_BUNDLE_IMAGE)
	 $(CONTAINER_CMD) push $(OPERATOR_BUNDLE_IMAGE)

build-operator-index-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run operator:forklift-operator-index-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD) \
		--action_env VERSION=$(VERSION) \
		--action_env CHANNELS=$(CHANNELS) \
		--action_env DEFAULT_CHANNEL=$(DEFAULT_CHANNEL) \
		--action_env OPT_OPTS=$(OPM_OPTS) \
		--action_env REGISTRY=$(REGISTRY) \
		--action_env REGISTRY_TAG=$(REGISTRY_TAG) \
		--action_env REGISTRY_ACCOUNT=$(REGISTRY_ACCOUNT)

push-operator-index-image: build-operator-index-image
	$(CONTAINER_CMD) tag bazel/operator:forklift-operator-index-image $(OPERATOR_INDEX_IMAGE)
	$(CONTAINER_CMD) push $(OPERATOR_INDEX_IMAGE)

build-populator-controller-image: check_container_runtmime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/populator-controller:populator-controller-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-populator-controller-image: build-populator-controller-image
	$(CONTAINER_CMD) tag bazel/cmd/populator-controller:populator-controller-image $(POPULATOR_CONTROLLER_IMAGE)
	$(CONTAINER_CMD) push $(POPULATOR_CONTROLLER_IMAGE)

# Build the docker image
build-ovirt-populator-image:
	$(CONTAINER_CMD) build -f hack/ovirt-populator/Containerfile -t $(OVIRT_POPULATOR_IMAGE) .

# Push the docker image
push-ovirt-populator-image: build-ovirt-populator-image
	$(CONTAINER_CMD) push $(OVIRT_POPULATOR_IMAGE)

build-openstack-populator-image: check_container_runtmime
	$(CONTAINER_CMD) build -f hack/openstack-populator/Containerfile -t $(OPENSTACK_POPULATOR_IMAGE) .

push-openstack-populator-image: build-openstack-populator-image
	$(CONTAINER_CMD) push $(OPENSTACK_POPULATOR_IMAGE)

build-all-images: build-api-image build-controller-image build-validation-image build-operator-image build-virt-v2v-image build-virt-v2v-warm-image build-operator-bundle-image build-operator-index-image build-populator-controller-image build-ovirt-populator-image build-openstack-populator-image

push-all-images: push-api-image push-controller-image push-validation-image push-operator-image push-virt-v2v-image push-virt-v2v-warm-image push-operator-bundle-image push-operator-index-image push-populator-controller-image push-ovirt-populator-image push-openstack-populator-image

.PHONY: check_container_runtmime
check_container_runtmime:
	@if [ ! -x "$(CONTAINER_CMD)" ]; then \
			echo "Container runtime was not automatically detected"; \
			echo "Please install podman or docker and make sure it's available in PATH"; \
			exit 1; \
	fi

bazel-generate:
	bazel run //:gazelle

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(DEFAULT_CONTROLLER_GEN):
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.10.0
