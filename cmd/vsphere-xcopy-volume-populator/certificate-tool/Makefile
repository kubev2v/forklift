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

# No longer need to use yq to read individual values for direct command line flags
# as the Go application will read the config file directly.
# These variables are kept if you use them elsewhere in the Makefile for other purposes,
# but they won't be passed as flags to the Go binary anymore for prepare/create-vm/test-xcopy.

# TEST_NAMESPACE = $(shell yq e '.test-namespace' $(CONFIG_FILE))
# STORAGE_VENDOR = $(shell yq e '.storage-vendor' $(CONFIG_FILE))
# KUBECONFIG = $(shell yq e '.kubeconfig' $(CONFIG_FILE))
# TEST_IMAGE_LABEL = $(shell yq e '.test-image-label' $(CONFIG_FILE))
# TEST_LABELS = $(shell yq e '.test-labels' $(CONFIG_FILE))
# TEST_POPULATOR_IMAGE = $(shell yq e '.test-populator-image' $(CONFIG_FILE))
# CONTROLLER_PATH = $(shell yq e '.controller-path' $(CONFIG_FILE))
# STORAGE_CLASS_NAME = $(shell yq e '.storage-class-name' $(CONFIG_FILE))
# SECRET_NAME = $(shell yq e '.secret-name' $(CONFIG_FILE))

# REQUIRED_VARS are no longer needed since parameters are read from the config file.
# You might keep this for other checks if needed, but for the Go tool's arguments, it's obsolete.
# REQUIRED_VARS := STORAGE_USERNAME STORAGE_PASSWORD STORAGE_HOSTNAME GOVMOMI_USERNAME GOVMOMI_PASSWORD GOVMOMI_HOSTNAME STORAGE_CLASS_NAME
# $(foreach var,$(REQUIRED_VARS),\
#   $(if $(value $(var)),,$(error Environment variable $(var) is required)))

# DATA_STORE can also be read from config, or kept for Makefile specific overrides
DATA_STORE ?= $(shell yq e '.vsphere-datastore' $(CONFIG_FILE))
BINARY = certificate-tool
VMDK_PATH ?= $(shell yq e '.local-vmdk-path' $(CONFIG_FILE)) # Read from config if it's there
.PHONY: help build prepare test-xcopy all clean create-vm destroy-vm

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

prepare: check-tools build ## Run certificate-tool prepare subcommand.
	@echo "running prepare..."
	@./$(BINARY) prepare --config "$(CONFIG_FILE)" # Pass the config file path

create-vm: build ## Run certificate-tool create-vm subcommand.
	@echo "creating vm..."
	./$(BINARY) create-vm --config "$(CONFIG_FILE)" # Pass the config file path

test-xcopy: build #prepare
	@echo "Running test-xcopy..."
	@./$(BINARY) test-xcopy \
		--config "$(CONFIG_FILE)" \
		--plan-yaml-path "assets/manifests/examples/example-test-plan.yaml" # Pass the config file and the plan file

destroy-vm: build
	@echo "destroying vm..."
	./$(BINARY) destroy-vm --config "$(CONFIG_FILE)" # Pass the config file path


all: create-vm test-xcopy ## Run all steps in sequence.

clean: ## Clean built artifacts.
	@rm -f $(BINARY)
	@$(KUBECTL) delete namespace $(shell yq e '.test-namespace' $(CONFIG_FILE))
	./$(BINARY) destroy-vm