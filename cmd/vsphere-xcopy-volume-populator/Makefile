# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: all
all: build

##@ Development
.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: generate 
generate:
	go generate ./...

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/vsphere-xcopy-volume-populator

# prerequisits: ensure a PVC exists.
test-copy-using-cli: build
	bin/vsphere-xcopy-volume-populator \
		--source-vmdk="[eco-iscsi-ds2] vm-1/vm-1.vmdk" \
		--target-pvc=test-cli \
		--target-namespace=default \
		--storage-vendor=ontap \
		--secret-name=populator-secret \
		--owner-uid=test-cli \
		--kubeconfig=$$KUBECONFIG

.PHONY: vmkfstools-wrapper
vmkfstools-wrapper:
	vmkfstools-wrapper/create-vib.sh
