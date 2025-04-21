SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

CONFIG_FILE ?= assets/config/static_values.yaml
ENV_FILE ?= assets/config/.env
-include $(ENV_FILE)

SHELL := /usr/bin/env bash

TEST_NAMESPACE = $(shell yq e '.test-namespace' $(CONFIG_FILE))
POD_NAMESPACE = $(shell yq e '.pod-namespace' $(CONFIG_FILE))
STORAGE_VENDOR   = $(shell yq e '.storage-vendor' $(CONFIG_FILE))
KUBECONFIG       = $(shell yq e '.kubeconfig' $(CONFIG_FILE))
TEST_IMAGE_LABEL = $(shell yq e '.test-image-label' $(CONFIG_FILE))
TEST_LABELS      = $(shell yq e '.test-labels' $(CONFIG_FILE))
TEST_POPULATOR_IMAGE = $(shell yq e '.test-populator-image' $(CONFIG_FILE))
CONTROLLER_PATH  = $(shell yq e '.controller-path' $(CONFIG_FILE))

REQUIRED_VARS := STORAGE_USER STORAGE_PASSWORD STORAGE_URL VSPHERE_USER VSPHERE_PASSWORD VSPHERE_URL

$(foreach var,$(REQUIRED_VARS),\
  $(if $(value $(var)),,$(error Environment variable $(var) is required)))


# TODO: pin the exact commit
OPENSTACK_POPULATOR ?= https://raw.githubusercontent.com/kubev2v/forklift/main/operator/config/crd/bases/forklift.konveyor.io_ovirtvolumepopulators.yaml
OVIRT_POPULATOR     ?= https://raw.githubusercontent.com/kubev2v/forklift/main/operator/config/crd/bases/forklift.konveyor.io_openstackvolumepopulators.yaml
VSPHERE_POPULATOR   ?= https://raw.githubusercontent.com/kubev2v/forklift/main/operator/config/crd/bases/forklift.konveyor.io_vspherexcopyvolumepopulators.yaml
CLUSTER_ROLE   ?= https://raw.githubusercontent.com/kubev2v/forklift/main/operator/config/rbac/populator-controller/role.yaml
ROLE_BINDING   ?= https://raw.githubusercontent.com/kubev2v/forklift/main/operator/config/rbac/populator-controller/role_binding.yaml
SERVICE_ACCOUNT   ?= https://raw.githubusercontent.com/kubev2v/forklift/main/operator/config/rbac/populator-controller/service_account.yaml
VM_NAME      ?= certificate-vm

BINARY = certificate-tool

.PHONY: help build prepare test-xcopy all clean

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

check-tools:
	@command -v yq >/dev/null || (echo "yq not installed" && exit 1)
	@command -v kubectl >/dev/null || (echo "kubectl not installed" && exit 1)
	@command -v go >/dev/null || (echo "go not installed" && exit 1)

build: ## Build the certificate-tool CLI binary.
	go build -o $(BINARY) .

prepare: check-tools build ## Run certificate-tool create-env subcommand.
	@kubectl apply -f $(OPENSTACK_POPULATOR)
	@kubectl apply -f $(OVIRT_POPULATOR)
	@kubectl apply -f $(VSPHERE_POPULATOR)
#	@kubectl apply -f $(CLUSTER_ROLE)
#	@kubectl apply -f $ROLE_BINDING)
                       ##	@kubectl apply -f $(SERVICE_ACCOUNT)
	./$(BINARY) prepare \
			 --kubeconfig            "$(KUBECONFIG)" \
			--test-namespace        "$(TEST_NAMESPACE)" \
			--test-image-label      "$(TEST_IMAGE_LABEL)" \
			--test-labels           "$(TEST_LABELS)" \
			--test-populator-image  "$(TEST_POPULATOR_IMAGE)" \
			--controller-path       "$(CONTROLLER_PATH)" \
			--pod-namespace         "$(POD_NAMESPACE)" \
			--test-image-label      "$(TEST_IMAGE_LABEL)" \
			--storage-user          "$(STORAGE_USER)" \
			--storage-password      "$(STORAGE_PASSWORD)" \
			--storage-url           "$(STORAGE_URL)" \
			--vsphere-user          "$(VSPHERE_USER)" \
			--vsphere-password      "$(VSPHERE_PASSWORD)" \
			--vsphere-url           "$(VSPHERE_URL)" \



create-vm: build ## Run certificate-tool create-vm subcommand.
	./$(BINARY) create-vm \
		--vm-name=$(VM_NAME) \
		--datastore=$(VM_NAME) \
		--vspherePassword=$(VSPHERE_PASSWORD) \
		--vsphereUrl=$(VSPHERE_URL) \
		--vsphereUser=$(VSPHERE_USER)

test-xcopy: build prepare
	@echo "Running test-xcopy..."
	./$(BINARY) test-xcopy \
		--kubeconfig $(KUBECONFIG) \
		--pvc-yaml "assets/manifests/xcopy-setup/xcopy-pvc.yaml"\
		--cr-yaml "assets/manifests/xcopy-setup/cr-test-xcopy.yaml" \
		--vmdk-path "[eco-iscsi-ds1] boris01/boris01.vmdk"  \
		--storage-vendor $(STORAGE_VENDOR)


all: create-vm test-xcopy ## Run all steps in sequence.

clean: ## Clean built artifacts.
	@rm -f $(BINARY)

