.SHELLFLAGS = -ec

CONFIG_FILE ?= assets/config/static_values.yaml
ENV_FILE ?= assets/config/.env
-include $(ENV_FILE)
GOPATH ?= `go env GOPATH`
GOBIN ?= $(GOPATH)/bin
# By default use the kubectl installed by the
# 'kubectl' target
DEFAULT_KUBECTL = $(GOBIN)/kubectl
KUBECTL ?= /usr/bin/kubectl


TEST_NAMESPACE = $(shell yq e '.test-namespace' $(CONFIG_FILE))
STORAGE_VENDOR   = $(shell yq e '.storage-vendor' $(CONFIG_FILE))
KUBECONFIG       = $(shell yq e '.kubeconfig' $(CONFIG_FILE))
TEST_IMAGE_LABEL = $(shell yq e '.test-image-label' $(CONFIG_FILE))
TEST_LABELS      = $(shell yq e '.test-labels' $(CONFIG_FILE))
TEST_POPULATOR_IMAGE = $(shell yq e '.test-populator-image' $(CONFIG_FILE))
CONTROLLER_PATH  = $(shell yq e '.controller-path' $(CONFIG_FILE))
STORAGE_CLASS_NAME  = $(shell yq e '.storage-class-name' $(CONFIG_FILE))
SECRET_NAME =  $(shell yq e '.secret-name' $(CONFIG_FILE))

REQUIRED_VARS := STORAGE_USER STORAGE_PASSWORD STORAGE_URL VSPHERE_USER VSPHERE_PASSWORD VSPHERE_URL STORAGE_CLASS_NAME

$(foreach var,$(REQUIRED_VARS),\
  $(if $(value $(var)),,$(error Environment variable $(var) is required)))

VM_NAME      ?= xcopy-popualtor-compliance-test-vm
DATA_STORE   ?= eco-iscsi-ds1
BINARY = certificate-tool
VMDK_PATH ?= "assets/cloudinit/fedora.vmdk"
.PHONY: help build prepare test-xcopy all clean

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

check-tools:
	@command -v yq >/dev/null || (echo "yq not installed" && exit 1)
	@command -v $(KUBECTL) >/dev/null || (echo "kubectl not installed" && exit 1)
	@command -v go >/dev/null || (echo "go not installed" && exit 1)

build: ## Build the certificate-tool CLI binary.
	go build -o $(BINARY) .

prepare: check-tools build ## Run certificate-tool create-env subcommand.
	@echo "running prepare..."
	@./$(BINARY) prepare \
			 --kubeconfig            "$(KUBECONFIG)" \
			--test-namespace        "$(TEST_NAMESPACE)" \
			--test-image-label      "$(TEST_IMAGE_LABEL)" \
			--test-labels           "$(TEST_LABELS)" \
			--test-populator-image  "$(TEST_POPULATOR_IMAGE)" \
			--controller-path       "$(CONTROLLER_PATH)" \
			--test-image-label      "$(TEST_IMAGE_LABEL)" \
			--storage-user          "$(STORAGE_USER)" \
			--storage-password      "$(STORAGE_PASSWORD)" \
			--storage-url           "$(STORAGE_URL)" \
			--vsphere-user          "$(VSPHERE_USER)" \
			--vsphere-password      "$(VSPHERE_PASSWORD)" \
			--vsphere-url           "$(VSPHERE_URL)"



create-vm: build ## Run certificate-tool create-vm subcommand.
	@echo "creating vm..."
	./$(BINARY) create-vm \
		--vm-name               $(VM_NAME) \
		--vsphere-user          "$(VSPHERE_USER)" \
		--vsphere-password      "$(VSPHERE_PASSWORD)" \
		--vsphere-url           "$(VSPHERE_URL)/sdk" \
		--data-center           "Eco-Datacenter" \
		--local-vmdk-path        $(VMDK_PATH) \
		--data-store             $(DATA_STORE)

test-xcopy: build #prepare
	@echo "Running test-xcopy..."
	@./$(BINARY) test-xcopy \
		--kubeconfig         	$(KUBECONFIG) \
		--pvc-yaml           	"assets/manifests/xcopy-setup/xcopy-pvc.yaml"\
		--storage-vendor-product     	$(STORAGE_VENDOR) \
		--storage-class-name 	$(STORAGE_CLASS_NAME) \
		--test-populator-image  $(TEST_POPULATOR_IMAGE) \
		--plan-yaml-path  "assets/manifests/examples/example-test-plan.yaml"


all: create-vm test-xcopy ## Run all steps in sequence.

clean: ## Clean built artifacts.
	@rm -f $(BINARY)
	@$(KUBECTL) delete namespace $(POD_NAMESPACE)
	./$(BINARY) destroy-vm \
		--vm-name 				$(VM_NAME) \
		--vsphere-user          "$(VSPHERE_USER)" \
		--vsphere-password      "$(VSPHERE_PASSWORD)" \
		--vsphere-url           "$(VSPHERE_URL)/sdk" \
		--data-center           "Eco-Datacenter" \
		--data-store             $(DATA_STORE)

