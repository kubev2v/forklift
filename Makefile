# Image URL to use all building/pushing image targets
IMG ?= quay.io/ocpmigrate/forklift-controller:latest
GOOS ?= `go env GOOS`
GOBIN ?= ${GOPATH}/bin
GO111MODULE = auto

ci: all

all: test manager

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager github.com/konveyor/forklift-controller/cmd/manager

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	export METRICS_PORT=8888;\
		go run ./cmd/manager/main.go

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
	${CONTROLLER_GEN} ${CRD_OPTIONS} crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crds/bases output:crd:dir=config/crds

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
#docker-build: test
docker-build:
	docker build . -t ${IMG}
	@echo "updating kustomize image patch file for manager resource"
	sed -i'' -e 's@image: .*@image: '"${IMG}"'@' ./config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
