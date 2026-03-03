# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# The build version which relies on release engineering setting tags that starts with 'v' like v2.10.4
VERSION := $(shell git describe --always --tags --match 'v*' 2>/dev/null || echo "v0.0.0-dev")
SCRIPT_VERSION := $(shell sh vmkfstools-wrapper/vmkfstools_wrapper.sh --version --output simple)

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
	GOFLAGS="-mod=mod" go generate ./...

LDFLAGS := -w -s \
		   -X 'github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/version.Version=$(VERSION)' \
		   -X 'github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/internal/version.VibVersion=$(VIB_VERSION)' \
		   -X 'github.com/kubev2v/forklift/cmd/vsphere-xcopy-volume-populator/vmkfstools-wrapper.Version=$(SCRIPT_VERSION)'
.PHONY: build
build: fmt vet
	$(eval SCRIPT_VERSION := $(shell sh vmkfstools-wrapper/vmkfstools_wrapper.sh --version --output simple))
	PATH="$(PATH)" \
	go build \
	    -ldflags="$(LDFLAGS)" \
	    -o bin/vsphere-xcopy-volume-populator \
	    vsphere-xcopy-volume-populator.go
	PATH="$(PATH)" \
	    go build \
	    -ldflags="-w -s" \
	    -o bin/vib-installer \
	    cmd/vib-installer.go

MOCKGEN := $(shell which mockgen 2>/dev/null)
install-mockgen:
	[ -z "$(MOCKGEN)" ] && go install go.uber.org/mock/mockgen@latest || true

# pre-requisits: ensure a PVC exists.
test-copy-using-cli: build
	bin/vsphere-xcopy-volume-populator \
		--source-vm-id=vm-92749 \
		--source-vmdk="[eco-iscsi-ds3] tshefi_40G/tshefi_40G.vmdk" \
		--owner-name=tshefi40g \
		--target-namespace=openshift-mtv \
		--storage-vendor-product=ontap \
		--secret-name=ontap \
		--owner-uid=tshefi40g \
		--esxi-clone-method=vib \
		--kubeconfig=$$KUBECONFIG

test-flashsystem: build
	bin/vsphere-xcopy-volume-populator \
		--source-vm-id=vm-48244 \
		--source-vmdk="[BorisO-tmp] BorisO-tmp/BorisO-tmp_3.vmdk" \
		--owner-name=test-cli2 \
		--target-namespace=flashsystem-test \
		--storage-vendor-product=flashsystem \
		--storage-skip-ssl-verification=true \
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
		--owner-name=test-vm-6 \
		--owner-uid=test-vm-6 \
		--target-namespace=default \
		--storage-vendor-product=pureFlashArray \
		--secret-name=pure \
		--kubeconfig=$$KUBECONFIG \
		--esxi-clone-method=vib


test-copy-using-cli-powerflex: build
	GOSCALEIO_DEBUG=1 bin/vsphere-xcopy-volume-populator \
		--source-vm-id=vm-122450 \
		--source-vmdk="[eco-dellpf-01] vm-powerflex/vm-powerflex.vmdk" \
		--owner-name=powerflex-4-8g \
		--target-namespace=default \
		--storage-vendor-product=powerflex \
		--secret-name=populator-secret \
		--kubeconfig=$$KUBECONFIG

test-copy-using-cli-powermax: build
	GOSCALEIO_DEBUG=1 bin/vsphere-xcopy-volume-populator \
		--source-vm-id="vm-1060" \
		--source-vmdk="[dlqa0051] testraghucentoswarm50x4/testraghucentoswarm50x4.vmdk" \
		--owner-name=test51g \
		--target-namespace=default \
		--storage-vendor-product=powermax \
		--secret-name=populator-secret \
		--kubeconfig=$$KUBECONFIG

test-copy-using-cli-infinibox: build
	bin/vsphere-xcopy-volume-populator \
		--source-vm-id="vm-143076" \
		--source-vmdk="[eco-iscsi-ds3] test-infinidat/test-infinidat.vmdk" \
		--owner-name=iscsi-block-pvc  \
		--target-namespace=infinidat-testing \
		--storage-vendor-product=infinibox \
		--secret-name=populator-secret \
		--kubeconfig=$$KUBECONFIG

test-copy-using-cli-powerstore: build extract-ssh-keys-from-cluster
	SSH_PRIVATE_KEY=$$(cat /tmp/test_ssh_private_key.b64) \
	SSH_PUBLIC_KEY=$$(cat /tmp/test_ssh_public_key.b64) \
	bin/vsphere-xcopy-volume-populator \
		--source-vm-id="fedora-vm-09" \
		--source-vmdk="[iscsi1] fedora-vm-09/small-vm2.vmdk" \
		--owner-name=repr-fedo-disk-0-2a9d22c9 \
		--esxi-clone-method=ssh \
		--target-namespace=default \
		--storage-vendor-product=powerstore \
		--secret-name=powerstore-secret \
		--kubeconfig=$$KUBECONFIG
.PHONY: vmkfstools-wrapper
vmkfstools-wrapper:
	$(MAKE) -C vmkfstools-wrapper build
