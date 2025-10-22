GOOS ?= $(shell go env GOOS)
GOPATH ?= $(shell go env GOPATH)
GOBIN ?= $(GOPATH)/bin
GO111MODULE = auto

ENVTEST_K8S_VERSION = 1.31.0
ENVTEST_VERSION ?= release-0.19

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

# Defaults for local development
VSPHERE_OS_MAP ?= forklift-virt-customize
OVIRT_OS_MAP ?= forklift-ovirt-osmap
VIRT_CUSTOMIZE_MAP ?= forklift-virt-customize
METRICS_PORT ?= 8888
METRICS_PORT_INVENTORY ?= 8889

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

# By default use the kustomize installed by the
# 'kustomize' target
DEFAULT_KUSTOMIZE = $(GOBIN)/kustomize
KUSTOMIZE ?= $(DEFAULT_KUSTOMIZE)

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
VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/vsphere-xcopy-volume-populator:$(REGISTRY_TAG)

### OLM
OPERATOR_BUNDLE_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-bundle:$(REGISTRY_TAG)
OPERATOR_INDEX_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-index:$(REGISTRY_TAG)

### External images
MUST_GATHER_IMAGE ?= quay.io/kubev2v/forklift-must-gather:latest
UI_PLUGIN_IMAGE ?= quay.io/kubev2v/forklift-console-plugin:latest

# Golangci-lint version
GOLANGCI_LINT_VERSION ?= v1.64.2
GOLANGCI_LINT_BIN ?= $(GOBIN)/golangci-lint

ci: all tidy vendor generate-verify lint

all: test forklift-controller

# Run tests
test: generate fmt vet manifests validation-test
	go test -coverprofile=cover.out ./pkg/... ./cmd/...

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
	go build -o bin/forklift-controller github.com/kubev2v/forklift/cmd/forklift-controller

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run
run: generate fmt vet
	VSPHERE_OS_MAP=$(VSPHERE_OS_MAP) \
	OVIRT_OS_MAP=$(OVIRT_OS_MAP) \
	VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE) \
	VIRT_CUSTOMIZE_MAP=$(VIRT_CUSTOMIZE_MAP) \
	METRICS_PORT=$(METRICS_PORT) \
	AUTH_REQUIRED=false \
		KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go run ./cmd/forklift-controller/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
.PHONY: run-inventory
run-inventory: generate fmt vet
	VSPHERE_OS_MAP=$(VSPHERE_OS_MAP) \
	OVIRT_OS_MAP=$(OVIRT_OS_MAP) \
	VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE) \
	VIRT_CUSTOMIZE_MAP=$(VIRT_CUSTOMIZE_MAP) \
	METRICS_PORT=$(METRICS_PORT_INVENTORY) \
	ROLE=inventory \
	AUTH_REQUIRED=false \
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

build-vsphere-xcopy-volume-populator-image: check_container_runtime
	$(eval VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE=$(REGISTRY)/$(REGISTRY_ORG)/vsphere-xcopy-volume-populator:$(REGISTRY_TAG))
	$(CONTAINER_CMD) build -t $(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE) -f build/vsphere-xcopy-volume-populator/Containerfile .

push-vsphere-xcopy-volume-populator-image: build-vsphere-xcopy-volume-populator-image
	$(CONTAINER_CMD) push $(VSPHERE_XCOPY_VOLUME_POPULATOR_IMAGE)

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
                  build-vsphere-xcopy-volume-populator-image\
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
                  push-vsphere-xcopy-volume-populator-image\
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
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.0

.PHONY: kubectl
kubectl: $(KUBECTL)
$(DEFAULT_KUBECTL):
	curl -L https://dl.k8s.io/release/v1.25.10/bin/linux/amd64/kubectl -o $(GOBIN)/kubectl && chmod +x $(GOBIN)/kubectl

.PHONY: kustomize
kustomize: $(KUSTOMIZE)
$(DEFAULT_KUSTOMIZE):
	go install sigs.k8s.io/kustomize/kustomize/v5@v5.3.0

validation-test: opa-bin
	ENVIRONMENT=test ${OPA} test validation/policies --explain fails

mockgen-install:
	go install go.uber.org/mock/mockgen@v0.4.0

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

# The directory where the 'crc' binary will be installed (this path
# will be added to the PATH variable). (default: ${HOME}/.local/bin)
CRC_BIN_DIR ?=
# Number of CPUS for CRC. By default all of the available CPUs will
# be used
CRC_CPUS ?= 8
# Memory for CRC in MB. (default: 16384)
CRC_MEM ?= 16384
# Disk size in GB. (default: 100)
CRC_DISK ?= 100
# Select openshift/okd installation type (default: okd)
CRC_PRESET ?= okd
# Pull secret file. If not provided it will be requested at
# installation time by the script
CRC_PULL_SECRET_FILE ?=
# Bundle to deploy. If not specified the default bundle will be
# installed. OKD default bundle doesn't work for now because of
# expired certificates so the installation script will temporarily
# overwrite it with:
# docker://quay.io/crcont/okd-bundle:4.13.0-0.okd-2023-06-04-080300
CRC_BUNDLE ?=
# Use the integrated CRC registry instead of local one. (default: '')
# Non empty variable is considered as true.
CRC_USE_INTEGRATED_REGISTRY ?=

install-crc:
	ROOTLESS=$(ROOTLESS) \
	CRC_BIN_DIR=$(CRC_BIN_DIR) \
	CRC_CPUS=$(CRC_CPUS) \
	CRC_MEM=$(CRC_MEM) \
	CRC_DISK=$(CRC_DISK) \
	CRC_PRESET=$(CRC_PRESET) \
	CRC_PULL_SECRET_FILE=$(CRC_PULL_SECRET_FILE) \
	CRC_BUNDLE=$(CRC_BUNDLE) \
	CRC_USE_INTEGRATED_REGISTRY=$(CRC_USE_INTEGRATED_REGISTRY) \
	./hack/installation/crc.sh;
	eval `crc oc-env`; \
	oc new-project "${REGISTRY_ORG}"

uninstall-crc:
	crc delete -f

# Driver: kvm2, docker or podman.
MINIKUBE_DRIVER ?= $(CONTAINER_RUNTIME)
MINIKUBE_CPUS ?= max
MINIKUBE_MEMORY ?= 16384
MINIKUBE_ADDONS ?= olm,kubevirt
MINIKUBE_USE_INTEGRATED_REGISTRY ?=

install-minikube:
	ROOTLESS=$(ROOTLESS) \
	MINIKUBE_DRIVER=$(MINIKUBE_DRIVER) \
	MINIKUBE_CPUS=$(MINIKUBE_CPUS) \
	MINIKUBE_MEMORY=$(MINIKUBE_MEMORY) \
	MINIKUBE_ADDONS=$(MINIKUBE_ADDONS) \
	MINIKUBE_USE_INTEGRATED_REGISTRY=$(MINIKUBE_USE_INTEGRATED_REGISTRY) \
	./hack/installation/minikube.sh

uninstall-minikube:
	minikube delete

ROOTLESS ?= true
# Kind version to install (default: v0.15.0)
KIND_VERSION ?= v0.15.0
# Kind operator Livecycle Manager version (default: v.0.25.0)
OLM_VERSION ?= v0.25.0
# Kind cert manager operator version (default: v1.12.2)
CERT_MANAGER_VERSION ?= v1.12.2

install-kind:
	ROOTLESS=$(ROOTLESS) \
	KIND_VERSION=$(KIND_VERSION) \
	OLM_VERSION=$(OLM_VERSION) \
	CERT_MANAGER_VERSION=$(CERT_MANAGER_VERSION) \
	./hack/installation/kind.sh; \
	[ $(CONTAINER_RUNTIME) != "podman" ] || export KIND_EXPERIMENTAL_PROVIDER="podman"; kind export kubeconfig --name forklift

uninstall-kind:
	[ $(CONTAINER_RUNTIME) != "podman" ] || export KIND_EXPERIMENTAL_PROVIDER="podman"; kind delete clusters forklift

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

# Deploy the operator and create a forklift controller in the configured Kubernetes cluster in ~/.kube/config
deploy: kubectl
	@echo -n "- Deploying to OKD: "
	@$(KUBECTL) get clusterrole system:image-puller &>/dev/null; OKD=$$?; \
	if [ $${OKD} -eq 0 ]; then echo "yes"; else echo "no"; fi; \
	echo "- Creating env files."; \
	for i in operator forklift rolebinding/{catalog,operator,default}; do \
		echo "$$DEPLOYMENT_VARS" > hack/deploy/$${i}/deploy.env; \
	done; \
	echo "- Creating the operator namespace: $(NAMESPACE)"; \
	$(KUBECTL) get namespace $(NAMESPACE) &>/dev/null || $(KUBECTL) create namespace $(NAMESPACE); \
	$(KUBECTL) get namespace $(CATALOG_NAMESPACE) &>/dev/null || $(KUBECTL) create namespace $(CATALOG_NAMESPACE); \
	$(KUBECTL) get namespace $(REGISTRY_ORG) &>/dev/null || $(KUBECTL) create namespace $(REGISTRY_ORG); \
	echo "- Creating the CatalogSource, OperatorGroup and the Subscription manifests"; \
	$(KUBECTL) apply -k hack/deploy/operator ; \
	if [ $$OKD -eq 0 ]; then \
		echo "- Creating the required RoleBindings for the deployment"; \
		$(KUBECTL) apply -k hack/deploy/rolebinding/default; \
		$(KUBECTL) apply -k hack/deploy/rolebinding/catalog ; \
	fi; \
	echo -n "- Waiting for the operator to be installed"; \
	until $(KUBECTL) -n $(NAMESPACE) get clusterserviceversion $(OPERATOR_NAME).v$(VERSION) &>/dev/null; do \
		sleep 1; echo -n "."; \
	done; \
	echo; \
	if [ $$OKD -eq 0 ]; then \
		echo "- Applying required role bindings"; \
		$(KUBECTL) apply -k hack/deploy/rolebinding/operator; \
	fi; \
	$(KUBECTL) -n $(NAMESPACE)  wait --timeout=60s --for=jsonpath=.status.phase=Succeeded clusterserviceversion $(OPERATOR_NAME).v$(VERSION); \
	echo "- Creating the Forklift Controller"; \
	$(KUBECTL) apply -k hack/deploy/forklift; \
	echo "Done!"

undeploy: kubectl
	@echo "- Removing the operator namespace: $(NAMESPACE)"
	@$(KUBECTL) get namespace $(NAMESPACE) -o name 2>/dev/null | xargs -r $(KUBECTL) delete ;
	@echo "- Removing the CatalogSource"
	@$(KUBECTL) get catalogsource -n $(CATALOG_NAMESPACE) -o name $(CATALOG_NAME) 2>/dev/null | xargs -r $(KUBECTL) -n $(CATALOG_NAMESPACE) delete;
	@echo "- Removing the Operator"
	@$(KUBECTL) get operator $(OPERATOR_NAME).$(NAMESPACE) -o name 2>/dev/null | xargs -r $(KUBECTL) delete;
	@echo "- Removing the Webhooks"
	@$(KUBECTL) get mutatingwebhookconfiguration forklift-api -o name 2>/dev/null | xargs -r $(KUBECTL) delete;
	@$(KUBECTL) get validatingwebhookconfiguration forklift-api -o name 2>/dev/null | xargs -r $(KUBECTL) delete;
	@echo "- Removing the ConsolePlugin"
	@$(KUBECTL) get consoleplugin forklift-console-plugin -o name 2>/dev/null | xargs -r $(KUBECTL) delete;
	@echo "- Removing the CRDs"
	@$(KUBECTL) get crd -l operators.coreos.com/forklift-operator.konveyor-forklift -o name 2>/dev/null | xargs -r $(KUBECTL) delete;
	@echo "- Removing the RoleBindings"
	@for ROLE_BINDING in forklift-{default,operator,controller,api,catalog,catalog-default} ; do \
		$(KUBECTL) -n $(REGISTRY_ORG) get rolebinding $${ROLE_BINDING} -o name 2>/dev/null | xargs -r $(KUBECTL) -n $(REGISTRY_ORG) delete ; \
	done;
	@echo "Done!"

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
    $(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

integration-test: generate fmt vet manifests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -i --bin-dir $(LOCALBIN) -p path)" go test ./pkg/controller/migration/... -coverprofile cover.out

build-controller:
	go build -o bin/forklift-controller cmd/forklift-controller/main.go

dev-controller: generate fmt vet build-controller
	ROLE="main" \
	API_HOST="forklift-inventory-openshift-mtv.apps.ocp-edge-cluster-0.qe.lab.redhat.com" \
	./bin/forklift-controller
	#dlv --listen=:5432 --headless=true --api-version=2 exec ./bin/forklift-controller \

.PHONY: kustomized-manifests
kustomized-manifests: kubectl
	kubectl kustomize operator/config/manifests > operator/.kustomized_manifests

.PHONY: generate-manifests
generate-manifests: kubectl manifests
	kubectl kustomize operator/streams/upstream > operator/streams/upstream/upstream_manifests
	kubectl kustomize operator/streams/downstream > operator/streams/downstream/downstream_manifests
	STREAM=upstream bash operator/streams/prepare-vars.sh
	STREAM=downstream bash operator/streams/prepare-vars.sh

.PHONY: lint-install
lint-install:
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	GOBIN=$(GOBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "golangci-lint installed successfully."

.PHONY: lint
lint: $(GOLANGCI_LINT_BIN)
	@echo "Running golangci-lint..."
	$(GOLANGCI_LINT_BIN) run ./pkg/... ./cmd/...

.PHONY: update-tekton
update-tekton:
	SKIP_UPDATE=false ./update-tekton.sh .tekton/*.yaml

$(GOLANGCI_LINT_BIN):
	$(MAKE) lint-install