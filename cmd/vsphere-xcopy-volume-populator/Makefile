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

PATH := $(shell go env GOPATH)/bin:$(PATH)

include vmkfstools-wrapper/version.mk

.PHONY: all
all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test: fmt vet
	go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: generate 
generate:
	go generate ./...

.PHONY: build
build: fmt vet
	PATH=$(PATH) \
	go build \
	-ldflags="-w -s -X github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/populator.VibVersion=$(VIB_VERSION)" \
	-o bin/vsphere-xcopy-volume-populator

MOCKGEN := $(shell which mockgen 2>/dev/null)
install-mockgen:
	[ -z "$(MOCKGEN)" ] && go install go.uber.org/mock/mockgen@latest || true

# pre-requisits: ensure a PVC exists.
test-copy-using-cli: build
	bin/vsphere-xcopy-volume-populator \
		--source-vm-id=vm-100838 \
		--source-vmdk="[eco-iscsi-ds3] vm-1-1-test-rename/vm-1-1-test-rename_2.vmdk" \
		--owner-name=test-cli \
		--target-namespace=default \
		--storage-vendor-product=ontap \
		--secret-name=populator-secret \
		--owner-uid=test-cli \
		--kubeconfig=$$KUBECONFIG

test-flashsystem: build
	bin/vsphere-xcopy-volume-populator \
		--source-vm-id=vm-1018 \
		--source-vmdk="[TEST_DS] test1-vm1/test1-vm1.vmdk" \
		--owner-name=test-cli \
		--target-namespace=default \
		--storage-vendor-product=flashsystem \
		--secret-name=populator-secret \
		--owner-uid=test-cli \
		--kubeconfig=$$KUBECONFIG

test-copy-using-cli-3par: build
	bin/vsphere-xcopy-volume-populator \
		--source-vmdk="[eco-iscsi-ds1] vm-6/vm-6.vmdk" \
	    --owner-name=test1\
		--target-namespace=default \
		--storage-vendor-product=primera3par \
		--secret-name=populator-secret \
		--kubeconfig=$$KUBECONFIG

.PHONY: vmkfstools-wrapper
vmkfstools-wrapper:
	$(MAKE) -C vmkfstools-wrapper build
