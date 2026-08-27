#!/bin/bash
set -eo pipefail

# Provisions a kind cluster running Calico OSS (without the aggregated
# calico-apiserver), then applies the Calico Enterprise projectcalico.org/v3
# CRDs for Network and IPPool (the api-server-less serving mode; the IPPool
# schema accepts allowedUses: L2Workload) plus sample resources. The result
# looks like a full-featured L2-capable Calico destination to Forklift's validation
# surface. Calico OSS stores the Enterprise resources but does not
# process them. This provides a quick, throwaway cluster for testing Forklift's
# role in the migration process, without bogging the process down with licensing
# and other Enterprise-only bootstrap steps.
#
# Usage: hack/calico/calico-mock-cluster.sh
#   CALICO_VERSION=v3.32.1 CLUSTER_NAME=forklift-calico-mock (defaults)
# Teardown: kind delete cluster --name forklift-calico-mock

# The Calico OSS version where the base installation comes from.
CALICO_VERSION="${CALICO_VERSION:-v3.32.1}"
CLUSTER_NAME="${CLUSTER_NAME:-forklift-calico-mock}"
KIND_VERSION="${KIND_VERSION:-v0.30.0}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN_DIR="${SCRIPT_DIR}/bin"
MOCK_DIR="${SCRIPT_DIR}"

# Ensure kind.
KIND="$(command -v kind || true)"
if [ -z "$KIND" ]; then
  mkdir -p "$BIN_DIR"
  KIND="$BIN_DIR/kind"
  if [ ! -x "$KIND" ]; then
    echo "Downloading kind ${KIND_VERSION}..."
    OS="$(uname | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"; case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64) ARCH=arm64 ;; esac
    curl -sLo "$KIND" "https://kind.sigs.k8s.io/dl/${KIND_VERSION}/kind-${OS}-${ARCH}"
    chmod +x "$KIND"
  fi
fi

echo "Creating kind cluster ${CLUSTER_NAME} (default CNI disabled)..."
cat <<EOF | "$KIND" create cluster --name "$CLUSTER_NAME" --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true
  podSubnet: 192.168.0.0/16
EOF

echo "Installing Calico OSS ${CALICO_VERSION} via the tigera operator..."
kubectl create -f "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/tigera-operator.yaml"
kubectl wait --for=condition=Available --timeout=300s deployment -n tigera-operator tigera-operator
echo "Waiting for the operator API to be established..."
until kubectl get crd installations.operator.tigera.io >/dev/null 2>&1; do sleep 2; done
kubectl wait --for=condition=Established --timeout=120s crd/installations.operator.tigera.io

# Apply only the Installation from custom-resources: no APIServer, so the
# projectcalico.org group stays free for the api-server-less CRDs below.
curl -sL "https://raw.githubusercontent.com/projectcalico/calico/${CALICO_VERSION}/manifests/custom-resources.yaml" \
  | python3 -c "import sys; print('\n---\n'.join(d for d in sys.stdin.read().split('---') if 'kind: Installation' in d))" \
  | kubectl create -f -

echo "Waiting for nodes and Calico to become ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s
kubectl -n calico-system rollout status daemonset/calico-node --timeout=300s
kubectl -n calico-system rollout status deployment/calico-kube-controllers --timeout=300s

echo "Applying the Enterprise Network and IPPool CRDs..."
kubectl apply --server-side --force-conflicts -f "${MOCK_DIR}/networks-crd.yaml"
kubectl apply --server-side --force-conflicts -f "${MOCK_DIR}/ippools-crd.yaml"

echo "Applying sample Enterprise-shaped resources..."
kubectl apply -f "${MOCK_DIR}/samples.yaml"

echo "Verifying Calico still works after the schema changes..."
kubectl -n calico-system rollout status daemonset/calico-node --timeout=120s
kubectl run mock-conncheck-a --image=busybox:1.36 --restart=Never -- sleep 300
kubectl run mock-conncheck-b --image=busybox:1.36 --restart=Never -- sleep 300
kubectl wait --for=condition=Ready pod/mock-conncheck-a pod/mock-conncheck-b --timeout=180s
B_IP="$(kubectl get pod mock-conncheck-b -o jsonpath='{.status.podIP}')"
kubectl exec mock-conncheck-a -- ping -c 3 -W 2 "$B_IP"
kubectl delete pod mock-conncheck-a mock-conncheck-b --wait=false

echo "Mock resources as stored:"
kubectl get networks.projectcalico.org mock-l2-net -o jsonpath='{.spec.l2Bridge.vlans[0]}'; echo
kubectl get ippools.projectcalico.org mock-vlan100-pool -o jsonpath='{.spec.allowedUses}'; echo

echo "API groups as Forklift's capability probe sees them:"
kubectl api-resources --api-group=projectcalico.org 2>/dev/null || true
kubectl api-resources --api-group=crd.projectcalico.org | grep -E "ippools|networks" || true

echo "Done. Cluster: ${CLUSTER_NAME} (kubectl context kind-${CLUSTER_NAME})"
