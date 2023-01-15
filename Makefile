REGISTRY ?= quay.io
REGISTRY_ACCOUNT ?= kubev2v
REGISTRY_TAG ?= devel
CONTROLLER_IMAGE ?= ${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-controller:${REGISTRY_TAG}
API_IMAGE ?= ${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-api:${REGISTRY_TAG}
VALIDATION_IMAGE ?= ${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-validation:${REGISTRY_TAG}
VIRT_V2V_IMAGE ?= ${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-virt-v2v:${REGISTRY_TAG}
OPERATOR_IMAGE ?= ${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-operator:${REGISTRY_TAG}
OPERATOR_BUNDLE_IMAGE ?= ${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-operator-bundle:${REGISTRY_TAG}
OPERATOR_INDEX_IMAGE ?= ${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-operator-index:${REGISTRY_TAG}
GOOS ?= `go env GOOS`
GOBIN ?= ${GOPATH}/bin
GO111MODULE = auto

ifeq (, $(shell which docker))
    CONTAINER_CMD = podman
else
    CONTAINER_CMD = docker
endif

ci: all

all: test forklift-controller

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Experimental e2e target
e2e-sanity:
	go test tests/base_test.go
	# vsphere suit
	go test ./tests/suit -v

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
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, Webhooks
manifests: controller-gen
	${CONTROLLER_GEN} crd rbac:roleName=manager-role webhook paths="./pkg/apis/..." output:dir=operator/config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate: controller-gen
	${CONTROLLER_GEN} object:headerFile="./hack/boilerplate.go.txt" paths="./pkg/apis/..."

build-controller-image:
	bazel run cmd/forklift-controller:forklift-controller-image

push-controller-image: build-controller-image
	$(CONTAINER_CMD) tag cmd/forklift-controller:forklift-controller-image ${CONTROLLER_IMAGE}
	$(CONTAINER_CMD) push ${CONTROLLER_IMAGE}

build-api-image:
	bazel run cmd/forklift-api:forklift-api-image

push-api-image: build-api-image
	$(CONTAINER_CMD) tag cmd/forklift-api:forklift-api-image ${API_IMAGE}
	$(CONTAINER_CMD) push ${API_IMAGE}

build-validation-image:
	bazel run validation:forklift-validation-image

push-validation-image: build-validation-image
	$(CONTAINER_CMD) tag validation:forklift-validation-image ${VALIDATION_IMAGE}
	$(CONTAINER_CMD) push ${VALIDATION_IMAGE}

build-operator-image:
	bazel run operator:forklift-operator-image

push-operator-image: build-operator-image
	$(CONTAINER_CMD) tag operator:forklift-operator-image ${OPERATOR_IMAGE}
	$(CONTAINER_CMD) push ${OPERATOR_IMAGE}

build-virt-v2v-image:
	bazel run virt-v2v:virt-v2v-image

push-virt-v2v-image: build-virt-v2v-image
	$(CONTAINER_CMD) tag virt-v2v:virt-v2v-image ${VIRT_V2V_IMAGE}
	$(CONTAINER_CMD) push ${VIRT_V2V_IMAGE}

build-operator-bundle-image:
	bazel run operator:forklift-operator-bundle-image --action_env CONTROLLER_IMAGE=${CONTROLLER_IMAGE} --action_env VALIDATION_IMAGE=${VALIDATION_IMAGE} --action_env OPERATOR_IMAGE=${OPERATOR_IMAGE} --action_env VIRT_V2V_IMAGE=${VIRT_V2v_IMAGE} --action_env API_IMAGE=${API_IMAGE}

push-operator-bundle-image: build-operator-bundle-image
	 $(CONTAINER_CMD) tag operator:forklift-operator-bundle-image ${OPERATOR_BUNDLE_IMAGE}
	 $(CONTAINER_CMD) push ${OPERATOR_BUNDLE_IMAGE}

build-operator-index-image:
	bazel run operator:forklift-operator-index-image --action_env REGISTRY=${REGISTRY} --action_env REGISTRY_ACCOUNT=${REGISTRY_ACCOUNT} --action_env REGISTRY_TAG=${REGISTRY_TAG}

push-operator-index-image: build-operator-index-image
	$(CONTAINER_CMD) tag operator:forklift-operator-index-image ${OPERATOR_INDEX_IMAGE}
	$(CONTAINER_CMD) push ${OPERATOR_INDEX_IMAGE}

push-all-images: push-api-image push-controller-image push-validation-image push-operator-image push-virt-v2v-image push-operator-bundle-image push-operator-index-image

bazel-generate:
	bazel run //:gazelle

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.10.0 ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
