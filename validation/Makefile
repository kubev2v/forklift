IMAGE_REGISTRY ?= quay.io
IMAGE_ORG ?= konveyor
IMAGE_REPO ?= forklift-validation
IMAGE_TAG ?= latest
IMAGE_FQIN ?= ${IMAGE_REGISTRY}/${IMAGE_ORG}/${IMAGE_REPO}:${IMAGE_TAG}

all: test

test: opa-bin
	ENVIRONMENT=test ${OPA} test policies --explain fails

docker-build:
	docker build -t ${IMAGE_FQIN} .

docker-push:
	docker push ${IMAGE_FQIN}

# Find or download opa
opa-bin:
ifeq (, $(shell which opa))
	@{ \
	set -e ;\
	mkdir -p ${HOME}/.local/bin ; \
	curl -sL -o ${HOME}/.local/bin/opa https://openpolicyagent.org/downloads/latest/opa_linux_amd64 ; \
	chmod 755 ${HOME}/.local/bin/opa ;\
	}
OPA=${HOME}/.local/bin/opa
else
OPA=$(shell which opa)
endif
