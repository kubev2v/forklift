# Image URL to use all building/pushing image targets
IMG ?= quay.io/ocpmigrate/forklift-controller:latest
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

# Build the docker image
build-controller:
	bazel run cmd/forklift-controller:forklift-controller-image

# Push the docker image
push-contoller: build-controller
	$(CONTAINER_CMD) tag cmd/forklift-controller:forklift-controller-image ${IMG}
	$(CONTAINER_CMD) push ${IMG}

# Build the docker image
build-api:
	bazel run cmd/forklift-api:forklift-api-image

# Push the docker image
push-api: build-api
	$(CONTAINER_CMD) tag cmd/forklift-api:forklift-api-image ${API_IMAGE}
	$(CONTAINER_CMD) push ${API_IMAGE}

# Build the docker image
build-operator:
	bazel run operator:forklift-operator-image

# Push the docker image
push-operator: build-operator
	$(CONTAINER_CMD) tag operator:forklift-operator-image ${OPERATOR_IMAGE}
	$(CONTAINER_CMD) push ${OPERATOR_IMAGE}

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
