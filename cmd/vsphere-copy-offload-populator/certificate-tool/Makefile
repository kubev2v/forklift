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

BINARY = certificate-tool
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

test-xcopy: build prepare
	@echo "Running test-xcopy..."
	@./$(BINARY) test-xcopy \
		--config "$(CONFIG_FILE)" \
		--plan-yaml-path "assets/manifests/examples/example-test-plan.yaml" # Pass the config file and the plan file

clean: ## Clean built artifacts.
	@rm -f $(BINARY)
	@$(KUBECTL) delete namespace $(shell yq e '.test-namespace' $(CONFIG_FILE))
	./$(BINARY) destroy-vm