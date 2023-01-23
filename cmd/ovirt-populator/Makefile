GOBIN ?= ${GOPATH}/bin
CONTROLLER_GEN=$(GOBIN)/controller-gen

manifests:
	${CONTROLLER_GEN} crd rbac:roleName=manager-role webhook paths="./pkg/..." output:crd:artifacts:config=config/crds/bases output:crd:dir=config/crds

generate:
	${CONTROLLER_GEN} object:headerFile="./hack/boilerplate.go.txt" paths="./..."
