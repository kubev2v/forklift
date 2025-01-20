GOOS ?= `go env GOOS`
GOPATH ?= `go env GOPATH`
GOBIN ?= $(GOPATH)/bin
GO111MODULE = auto


CONTAINER_RUNTIME ?=

ifeq ($(CONTAINER_RUNTIME),)
CONTAINER_CMD ?= $(shell type -P podman)
ifeq ($(CONTAINER_CMD),)
CONTAINER_CMD := $(shell type -P docker)
endif
CONTAINER_RUNTIME=$(shell basename $(CONTAINER_CMD))
else
CONTAINER_CMD := $(shell type -P $(CONTAINER_RUNTIME))
endif

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

# Use OPM_OPTS="--use-http" when using a non HTTPS registry
# Use OPM_OPTS="--skip-tls-verify" when using an HTTPS registry with self-signed certificate
OPM_OPTS ?=

# By default use the controller gen installed by the
# 'controller-gen' target
DEFAULT_CONTROLLER_GEN = $(GOBIN)/controller-gen
CONTROLLER_GEN ?= $(DEFAULT_CONTROLLER_GEN)

# By default use the kubectl installed by the
# 'kubectl' target
DEFAULT_KUBECTL = $(GOBIN)/kubectl
KUBECTL ?= $(DEFAULT_KUBECTL)

# Image URLs to use all building/pushing image targets
# Each build image target overrides the variable of that image
# This is used for the bundle build so we don't need to build all images
# So the bundle will point to latest available images and to those which were built.
# Example: REGISTRY_ORG=mnecas0 make build-controller-image build-operator-bundle-image
# This will build controller and bundle pointing to that controller

### Components
CONTROLLER_IMAGE ?= quay.io/kubev2v/forklift-controller:latest
API_IMAGE ?= quay.io/kubev2v/forklift-api:latest
VALIDATION_IMAGE ?= quay.io/kubev2v/forklift-validation:latest
VIRT_V2V_IMAGE ?= quay.io/kubev2v/forklift-virt-v2v:latest
OPERATOR_IMAGE ?= quay.io/kubev2v/forklift-operator:latest
POPULATOR_CONTROLLER_IMAGE ?= quay.io/kubev2v/populator-controller:latest
OVIRT_POPULATOR_IMAGE ?= quay.io/kubev2v/ovirt-populator:latest
OPENSTACK_POPULATOR_IMAGE ?= quay.io/kubev2v/openstack-populator:latest
OVA_PROVIDER_SERVER_IMAGE ?= quay.io/kubev2v/forklift-ova-provider-server:latest

### OLM
OPERATOR_BUNDLE_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-bundle:$(REGISTRY_TAG)
OPERATOR_INDEX_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-index:$(REGISTRY_TAG)

### External images
MUST_GATHER_IMAGE ?= quay.io/kubev2v/forklift-must-gather:latest
UI_PLUGIN_IMAGE ?= quay.io/kubev2v/forklift-console-plugin:latest

ci: all tidy vendor generate-verify

all: test forklift-controller

# Run tests
test: generate fmt vet manifests validation-test
	go test -coverprofile=cover.out ./pkg/... ./cmd/... ./virt-v2v/...

# Experimental e2e target
e2e-sanity: e2e-sanity-ovirt e2e-sanity-vsphere

e2e-sanity-ovirt:
	# ovirt suite
	KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go test ./tests/suit -v -ginkgo.focus ".*oVirt.*|.*Forklift.*"

e2e-sanity-vsphere:
	# vsphere suit
	go test ./tests/suit -v -ginkgo.focus ".*vSphere.*"

e2e-sanity-openstack:
	# openstack suit
	go test ./tests/suit -v -ginkgo.focus ".*Migration tests for OpenStack.*"

e2e-sanity-openstack-extended:
	# openstack extended suit
	sudo bash -c  'echo "127.0.0.1 packstack.konveyor-forklift" >>/etc/hosts'
	go test ./tests/suit -v -ginkgo.focus ".*Migration Extended tests for OpenStack.*" -ginkgo.parallel.total 1

e2e-sanity-ova:
	# ova suit
	go test ./tests/suit -v -ginkgo.focus ".*OVA.*"


# Build forklift-controller binary
forklift-controller: generate fmt vet
	go build -o bin/forklift-controller github.com/konveyor/forklift-controller/cmd/forklift-controller

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	export METRICS_PORT=8888;\
		KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go run ./cmd/forklift-controller/main.go

# Install CRDs into a cluster
install: manifests kubectl
	$(KUBECTL) apply -k operator/config/crd

# Generate manifests e.g. CRD, Webhooks
manifests: controller-gen
	$(CONTROLLER_GEN) crd rbac:roleName=manager-role webhook paths="./pkg/apis/..." output:dir=operator/config/crd/bases

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

build-controller-image: check_container_runtime
	$(eval CONTROLLER_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-controller:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(CONTROLLER_IMAGE) -f build/forklift-controller/Containerfile .

push-controller-image: build-controller-image
	$(CONTAINER_CMD) push $(CONTROLLER_IMAGE)

build-api-image: check_container_runtime
	$(eval API_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-api:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(API_IMAGE) -f build/forklift-api/Containerfile .

push-api-image: build-api-image
	$(CONTAINER_CMD) push $(API_IMAGE)

build-validation-image: check_container_runtime
	$(eval VALIDATION_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-validation:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(VALIDATION_IMAGE) -f build/validation/Containerfile .

push-validation-image: build-validation-image
	$(CONTAINER_CMD) push $(VALIDATION_IMAGE)

build-operator-image: check_container_runtime
	$(eval OPERATOR_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-operator:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(OPERATOR_IMAGE) -f build/forklift-operator/Containerfile .

push-operator-image: build-operator-image
	$(CONTAINER_CMD) push $(OPERATOR_IMAGE)

build-virt-v2v-image: check_container_runtime
	$(eval VIRT_V2V_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-virt-v2v:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(VIRT_V2V_IMAGE) -f build/virt-v2v/Containerfile-upstream .

push-virt-v2v-image: build-virt-v2v-image
	$(CONTAINER_CMD) push $(VIRT_V2V_IMAGE)

build-operator-bundle-image: check_container_runtime
	$(CONTAINER_CMD) build \
		-t $(OPERATOR_BUNDLE_IMAGE) \
		-f build/forklift-operator-bundle/Containerfile . \
		--build-arg STREAM=dev \
		--build-arg VERSION=$(VERSION) \
		--build-arg CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) \
		--build-arg API_IMAGE=$(API_IMAGE) \
		--build-arg VALIDATION_IMAGE=$(VALIDATION_IMAGE) \
		--build-arg VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE) \
		--build-arg OPERATOR_IMAGE=$(OPERATOR_IMAGE) \
		--build-arg POPULATOR_CONTROLLER_IMAGE=$(POPULATOR_CONTROLLER_IMAGE) \
		--build-arg OVIRT_POPULATOR_IMAGE=$(OVIRT_POPULATOR_IMAGE) \
		--build-arg OPENSTACK_POPULATOR_IMAGE=$(OPENSTACK_POPULATOR_IMAGE) \
		--build-arg MUST_GATHER_IMAGE=$(MUST_GATHER_IMAGE) \
		--build-arg UI_PLUGIN_IMAGE=$(UI_PLUGIN_IMAGE) \
		--build-arg OVA_PROVIDER_SERVER_IMAGE=$(OVA_PROVIDER_SERVER_IMAGE)


push-operator-bundle-image: build-operator-bundle-image
	 $(CONTAINER_CMD) push $(OPERATOR_BUNDLE_IMAGE)

build-operator-index-image: check_container_runtime
	$(eval OPERATOR_INDEX_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-index:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build $(BUILD_OPT) -t $(OPERATOR_INDEX_IMAGE) -f build/forklift-operator-index/Containerfile . \
		--build-arg VERSION=$(VERSION) \
		--build-arg OPERATOR_BUNDLE_IMAGE=$(OPERATOR_BUNDLE_IMAGE) \
		--build-arg CHANNELS=$(CHANNELS) \
		--build-arg DEFAULT_CHANNEL=$(DEFAULT_CHANNEL) \
		--build-arg OPM_OPTS=$(OPM_OPTS)

push-operator-index-image: build-operator-index-image
	$(CONTAINER_CMD) push $(OPERATOR_INDEX_IMAGE)

build-populator-controller-image: check_container_runtime
	$(eval POPULATOR_CONTROLLER_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/populator-controller:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(POPULATOR_CONTROLLER_IMAGE) -f build/populator-controller/Containerfile .

push-populator-controller-image: build-populator-controller-image
	$(CONTAINER_CMD) push $(POPULATOR_CONTROLLER_IMAGE)

build-ovirt-populator-image:
	$(eval OVIRT_POPULATOR_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/ovirt-populator:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(OVIRT_POPULATOR_IMAGE) -f build/ovirt-populator/Containerfile-upstream .

push-ovirt-populator-image: build-ovirt-populator-image
	$(CONTAINER_CMD) push $(OVIRT_POPULATOR_IMAGE)

build-openstack-populator-image: check_container_runtime
	$(eval OPENSTACK_POPULATOR_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/openstack-populator:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(OPENSTACK_POPULATOR_IMAGE) -f build/openstack-populator/Containerfile .

push-openstack-populator-image: build-openstack-populator-image
	$(CONTAINER_CMD) push $(OPENSTACK_POPULATOR_IMAGE)

build-ova-provider-server-image: check_container_runtime
	$(eval OVA_PROVIDER_SERVER_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/forklift-ova-provider-server:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(OVA_PROVIDER_SERVER_IMAGE) -f build/ova-provider-server/Containerfile .

push-ova-provider-server-image: build-ova-provider-server-image
	$(CONTAINER_CMD) push $(OVA_PROVIDER_SERVER_IMAGE)

build-all-images: build-api-image \
                  build-controller-image \
                  build-validation-image \
                  build-operator-image \
                  build-virt-v2v-image \
                  build-populator-controller-image \
                  build-ovirt-populator-image \
                  build-openstack-populator-image\
                  build-ova-provider-server-image \
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
                  push-ova-provider-server-image \
                  push-operator-bundle-image \
                  push-operator-index-image


.PHONY: deploy-operator-index
deploy-operator-index:
	export OPERATOR_INDEX_IMAGE=${OPERATOR_INDEX_IMAGE}; envsubst < operator/forklift-operator-catalog.yaml | kubectl apply -f -

.PHONY: check_container_runtime
check_container_runtime:
	@if [ ! -x "$(CONTAINER_CMD)" ]; then \
			echo "Container runtime was not automatically detected"; \
			echo "Please install podman or docker and make sure it's available in PATH"; \
			exit 1; \
	fi

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN)
$(DEFAULT_CONTROLLER_GEN):
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.15.0

.PHONY: kubectl
kubectl: $(KUBECTL)
$(DEFAULT_KUBECTL):
	curl -L https://dl.k8s.io/release/v1.25.10/bin/linux/amd64/kubectl -o $(GOBIN)/kubectl && chmod +x $(GOBIN)/kubectl

validation-test: opa-bin
	ENVIRONMENT=test ${OPA} test validation/policies --explain fails

opa-bin:
ifeq (, $(shell command -v opa))
	@{ \
	set -e ;\
	mkdir -p ${HOME}/.local/bin ; \
	curl -sL -o ${HOME}/.local/bin/opa https://openpolicyagent.org/downloads/v0.65.0/opa_linux_amd64_static ; \
	chmod 755 ${HOME}/.local/bin/opa ;\
	}
OPA=${HOME}/.local/bin/opa
else
OPA=$(shell which opa)
endif
