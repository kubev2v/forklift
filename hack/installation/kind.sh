#! /bin/bash

KIND_VERSION="${KIND_VERSION:-v0.15.0}"

OLM_VERSION="${OLM_VERSION:-v0.25.0}"
OLM_INSTALL_URL="https://github.com/operator-framework/operator-lifecycle-manager/releases/download/${OLM_VERSION}/install.sh"

CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.12.2}"
CERT_MANAGER_URL="https://github.com/jetstack/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"

KUBEVIRT_VERSION="${KUBEVIRT_VERSION:-$(curl -s https://api.github.com/repos/kubevirt/kubevirt/releases | grep tag_name | grep -v -- '-rc' | sort -r | head -1 | awk -F': ' '{print $2}' | sed 's/,//' | xargs)}"
KUBEVIRT_URL="https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"

CONTAINER_RUNTIME="${CONTAINER_RUNTIME}"
if [ -z "${CONTAINER_RUNTIME}" ]; then
  CONTAINER_CMD="${CONTAINER_CMD:-$(type -P podman || type -P docker || :)}"
  if [ -z "${CONTAINER_CMD}" ]; then
    echo "Container runtime not detected"
    exit 1
  fi
  CONTAINER_RUNTIME="$(basename ${CONTAINER_CMD})"
else
  CONTAINER_CMD=$(type -P $CONTAINER_RUNTIME)
fi

export REGISTRY_IP=localhost
export REGISTRY_PORT="${REGISTRY_PORT:-5001}"

export LOCAL_REGISTRY_NAME="${LOCAL_REGISTRY_NAME:-forklift-registry}"
export LOCAL_REGISTRY_IP="${LOCAL_REGISTRY_IP:-localhost}"
export LOCAL_REGISTRY_PORT="${LOCAL_REGISTRY_PORT:-5000}"

[ "$(type -P go )" ] || ( echo "go is not in PATH" ;  exit 2 )
go install "sigs.k8s.io/kind@${KIND_VERSION}"

[ "$(type -P kind)" ] || ( echo "kind is not in PATH" ;  exit 2 )

if [ "${CONTAINER_RUNTIME}" == "podman" ]; then
  export KIND_EXPERIMENTAL_PROVIDER="podman"
  export ROOTLESS="true"
fi

if [ "${CONTAINER_RUNTIME}" == "docker" -a "${ROOTLESS}" ]; then
  echo "Setting up docker rootless"
  [ "$(${CONTAINER_CMD} context ls --format json | jq -r '. | select(.Name == "rootless").Name')" == "rootless" ] || dockerd-rootless-setuptool.sh install -f
  docker context use rootless
fi

# 1. create registry container unless it already exists
$(dirname -- ${BASH_SOURCE[0]})/registry.sh

# 2. Create kind cluster
cat <<EOF | kind create cluster --name forklift --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_IP}:${REGISTRY_PORT}"]
    endpoint = ["http://${LOCAL_REGISTRY_NAME}:${LOCAL_REGISTRY_PORT}"]
EOF

# 3. Connect the registry to the cluster network if not already connected
# This allows kind to bootstrap the network but ensures they're on the same network
if [ "$(${CONTAINER_CMD} inspect -f='{{json .NetworkSettings.Networks.kind}}' "${LOCAL_REGISTRY_NAME}")" = 'null' ]; then
  ${CONTAINER_CMD} network connect "kind" "${LOCAL_REGISTRY_NAME}"
fi

# 4. Document the local registry
# https://github.com/kubernetes/enhancements/tree/master/keps/sig-cluster-lifecycle/generic/1755-communicating-a-local-registry
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "${REGISTRY_IP}:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

# 5. Install the Operator Lifecycle Manager
curl -sL "${OLM_INSTALL_URL}" | sh -s -- ${OLM_VERSION}
# 6. Install the Cert Manager
kubectl apply -f "${CERT_MANAGER_URL}"
kubectl apply -f "${KUBEVIRT_URL}"
