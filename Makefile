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

all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/konveyor/forklift-controller/cmd/manager

# Build manager binary with compiler optimizations disabled
debug: generate fmt vet
	go build -o bin/manager -gcflags=all="-N -l" github.com/konveyor/forklift-controller/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	export METRICS_PORT=8888;\
		KUBEVIRT_CLIENT_GO_SCHEME_REGISTRATION_VERSION=v1 go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

CRD_OPTIONS ?= "crd:trivialVersions=true"

# Generate manifests e.g. CRD, Webhooks
manifests: controller-gen
	${CONTROLLER_GEN} ${CRD_OPTIONS} crd rbac:roleName=manager-role webhook paths="./..." output:dir=operator/config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate: controller-gen
	${CONTROLLER_GEN} object:headerFile="./hack/boilerplate.go.txt" paths="./..."

# Build the docker image
build-controller:
	bazel run cmd/manager:forklift-controller-image

# Push the docker image
push-contoller: build-controller
	$(CONTAINER_CMD) tag cmd/manager:forklift-controller-image ${IMG}
	$(CONTAINER_CMD) push ${IMG}

bazel-generate:
	bazel run //:gazelle

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.2 ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
