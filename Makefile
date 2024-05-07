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

VERSION ?= 2.6.2
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
CONTROLLER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-controller:$(REGISTRY_TAG)
API_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-api:$(REGISTRY_TAG)
VALIDATION_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-validation:$(REGISTRY_TAG)
VIRT_V2V_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-virt-v2v:$(REGISTRY_TAG)
VIRT_V2V_WARM_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-virt-v2v-warm:$(REGISTRY_TAG)
OPERATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator:$(REGISTRY_TAG)
OPERATOR_BUNDLE_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-bundle:$(REGISTRY_TAG)
OPERATOR_INDEX_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-operator-index:$(REGISTRY_TAG)
POPULATOR_CONTROLLER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/populator-controller:$(REGISTRY_TAG)
OVIRT_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/ovirt-populator:$(REGISTRY_TAG)
OPENSTACK_POPULATOR_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/openstack-populator:$(REGISTRY_TAG)
OVA_PROVIDER_SERVER_IMAGE ?= $(REGISTRY)/$(REGISTRY_ORG)/forklift-ova-provider-server:$(REGISTRY_TAG)

### External images
MUST_GATHER_IMAGE ?= quay.io/kubev2v/forklift-must-gather:latest
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

ci: all tidy vendor bazel-generate generate-verify

all: test forklift-controller

# Run tests
test: generate fmt vet manifests validation-test
	go test -coverprofile=cover.out ./pkg/... ./cmd/... ./virt-v2v/cold/...

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

# Build manager binary with compiler optimizations disabled
debug: generate fmt vet
	go build -o bin/forklift-controller -gcflags=all="-N -l" github.com/konveyor/forklift-controller/cmd/forklift-controller

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	export METRICS_PORT=8888;\
		KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go run ./cmd/forklift-controller/main.go

# Install CRDs into a cluster
install: manifests kubectl
	$(KUBECTL) apply -k operator/config/crds

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
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/forklift-controller:forklift-controller-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-controller-image: build-controller-image
	$(CONTAINER_CMD) tag bazel/cmd/forklift-controller:forklift-controller-image $(CONTROLLER_IMAGE)
	$(CONTAINER_CMD) push $(CONTROLLER_IMAGE)

build-api-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/forklift-api:forklift-api-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-api-image: build-api-image
	$(CONTAINER_CMD) tag bazel/cmd/forklift-api:forklift-api-image $(API_IMAGE)
	$(CONTAINER_CMD) push $(API_IMAGE)

build-validation-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run validation:forklift-validation-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-validation-image: build-validation-image
	$(CONTAINER_CMD) tag bazel/validation:forklift-validation-image $(VALIDATION_IMAGE)
	$(CONTAINER_CMD) push $(VALIDATION_IMAGE)

build-operator-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run operator:forklift-operator-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-operator-image: build-operator-image
	$(CONTAINER_CMD) tag bazel/operator:forklift-operator-image $(OPERATOR_IMAGE)
	$(CONTAINER_CMD) push $(OPERATOR_IMAGE)

build-virt-v2v-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run --package_path=virt-v2v/cold forklift-virt-v2v \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-virt-v2v-image: build-virt-v2v-image
	$(CONTAINER_CMD) tag bazel:forklift-virt-v2v $(VIRT_V2V_IMAGE)
	$(CONTAINER_CMD) push $(VIRT_V2V_IMAGE)

build-virt-v2v-warm-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run --package_path=virt-v2v/warm forklift-virt-v2v-warm \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-virt-v2v-warm-image: build-virt-v2v-warm-image
	$(CONTAINER_CMD) tag bazel:forklift-virt-v2v-warm ${VIRT_V2V_WARM_IMAGE}
	$(CONTAINER_CMD) push ${VIRT_V2V_WARM_IMAGE}

build-operator-bundle-image: check_container_runtime
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
		--action_env UI_PLUGIN_IMAGE=$(UI_PLUGIN_IMAGE) \
		--action_env VALIDATION_IMAGE=$(VALIDATION_IMAGE) \
		--action_env VIRT_V2V_IMAGE=$(VIRT_V2V_IMAGE) \
		--action_env VIRT_V2V_WARM_IMAGE=$(VIRT_V2V_WARM_IMAGE) \
		--action_env CONTROLLER_IMAGE=$(CONTROLLER_IMAGE) \
		--action_env API_IMAGE=$(API_IMAGE) \
		--action_env POPULATOR_CONTROLLER_IMAGE=$(POPULATOR_CONTROLLER_IMAGE) \
		--action_env OVIRT_POPULATOR_IMAGE=$(OVIRT_POPULATOR_IMAGE) \
		--action_env OPENSTACK_POPULATOR_IMAGE=$(OPENSTACK_POPULATOR_IMAGE)\
		--action_env OVA_PROVIDER_SERVER_IMAGE=$(OVA_PROVIDER_SERVER_IMAGE)

push-operator-bundle-image: build-operator-bundle-image
	 $(CONTAINER_CMD) tag bazel/operator:forklift-operator-bundle-image $(OPERATOR_BUNDLE_IMAGE)
	 $(CONTAINER_CMD) push $(OPERATOR_BUNDLE_IMAGE)

build-operator-index-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run operator:forklift-operator-index-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD) \
		--action_env VERSION=$(VERSION) \
		--action_env CHANNELS=$(CHANNELS) \
		--action_env DEFAULT_CHANNEL=$(DEFAULT_CHANNEL) \
		--action_env OPM_OPTS=$(OPM_OPTS) \
		--action_env REGISTRY=$(REGISTRY) \
		--action_env REGISTRY_TAG=$(REGISTRY_TAG) \
		--action_env REGISTRY_ORG=$(REGISTRY_ORG)

push-operator-index-image: build-operator-index-image
	$(CONTAINER_CMD) tag bazel/operator:forklift-operator-index-image $(OPERATOR_INDEX_IMAGE)
	$(CONTAINER_CMD) push $(OPERATOR_INDEX_IMAGE)

build-populator-controller-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/populator-controller:populator-controller-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-populator-controller-image: build-populator-controller-image
	$(CONTAINER_CMD) tag bazel/cmd/populator-controller:populator-controller-image $(POPULATOR_CONTROLLER_IMAGE)
	$(CONTAINER_CMD) push $(POPULATOR_CONTROLLER_IMAGE)

build-ovirt-populator-image:
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/ovirt-populator:ovirt-populator-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-ovirt-populator-image: build-ovirt-populator-image
	$(CONTAINER_CMD) tag bazel/cmd/ovirt-populator:ovirt-populator-image $(OVIRT_POPULATOR_IMAGE)
	$(CONTAINER_CMD) push $(OVIRT_POPULATOR_IMAGE)

build-openstack-populator-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/openstack-populator:openstack-populator-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-openstack-populator-image: build-openstack-populator-image
	$(CONTAINER_CMD) tag bazel/cmd/openstack-populator:openstack-populator-image $(OPENSTACK_POPULATOR_IMAGE)
	$(CONTAINER_CMD) push $(OPENSTACK_POPULATOR_IMAGE)

build-ova-provider-server-image: check_container_runtime
	export CONTAINER_CMD=$(CONTAINER_CMD); \
	bazel run cmd/ova-provider-server:ova-provider-server-image \
		$(BAZEL_OPTS) \
		--action_env CONTAINER_CMD=$(CONTAINER_CMD)

push-ova-provider-server-image: build-ova-provider-server-image
	$(CONTAINER_CMD) tag bazel/cmd/ova-provider-server:ova-provider-server-image $(OVA_PROVIDER_SERVER_IMAGE)
	$(CONTAINER_CMD) push $(OVA_PROVIDER_SERVER_IMAGE)

build-all-images: build-api-image \
                  build-controller-image \
                  build-validation-image \
                  build-operator-image \
                  build-virt-v2v-image \
                  build-virt-v2v-warm-image \
                  build-operator-bundle-image \
                  build-operator-index-image \
                  build-populator-controller-image \
                  build-ovirt-populator-image \
                  build-openstack-populator-image\
                  build-ova-provider-server-image

push-all-images:  push-api-image \
                  push-controller-image \
                  push-validation-image \
                  push-operator-image \
                  push-virt-v2v-image \
                  push-virt-v2v-warm-image \
                  push-operator-bundle-image \
                  push-operator-index-image \
                  push-populator-controller-image \
                  push-ovirt-populator-image \
                  push-openstack-populator-image\
                  push-ova-provider-server-image

.PHONY: check_container_runtime
check_container_runtime:
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
	curl -sL -o ${HOME}/.local/bin/opa https://openpolicyagent.org/downloads/v0.57.1/opa_linux_amd64_static ; \
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
