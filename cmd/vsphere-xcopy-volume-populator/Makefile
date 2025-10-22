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


test-copy-using-cli-3par: build
	bin/vsphere-xcopy-volume-populator \
		--source-vmdk="[eco-iscsi-ds1] vm-8/vm-8.vmdk" \
		--owner-name=3par-test-100g \
		--owner-uid=3par-test-100g \
		--target-namespace=default \
		--storage-vendor-product=primera3par \
		--secret-name=populator-secret \
		--storage-skip-ssl-verification=true \
		--kubeconfig=$$KUBECONFIG

test-copy-using-cli-pureflasharray: build
	bin/vsphere-xcopy-volume-populator \
		--source-vm-id="vm-67218" \
		--source-vmdk="[eco-iscsi-pure] vm-6/vm-6.vmdk" \
		--owner-name=test-pure-vm-6-22ab1d65 \
		--owner-uid=test-pure-vm-6-22ab1d65 \
		--target-namespace=default \
		--storage-vendor-product=pureFlashArray \
		--secret-name=populator-secret \
		--kubeconfig=$$KUBECONFIG

test-copy-using-cli-powerflex: build
	GOSCALEIO_DEBUG=1 bin/vsphere-xcopy-volume-populator \
		--source-vm-id=vm-122450 \
		--source-vmdk="[eco-dellpf-01] vm-powerflex/vm-powerflex.vmdk" \
		--owner-name=powerflex-4-8g \
		--target-namespace=default \
		--storage-vendor-product=powerflex \
		--secret-name=populator-secret \
		--kubeconfig=$$KUBECONFIG

.PHONY: vmkfstools-wrapper
vmkfstools-wrapper:
	$(MAKE) -C vmkfstools-wrapper build
