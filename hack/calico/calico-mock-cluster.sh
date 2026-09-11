#!/bin/bash
set -eo pipefail

# Provisions a kind cluster running Calico OSS in api-server-less mode: the
# projectcalico.org/v3 API is served by CRDs (no aggregated calico-apiserver),
# and is Calico's active datastore. On top of that base, the Calico Enterprise
# CRD schemas are applied for every resource Forklift reads — Network, IPPool,
# FelixConfiguration, BGPPeer — so the cluster stores all CRDs that Forklift
# may read across all Calico variants. Enterprise-only fields
# are stored but have no OSS behavior behind them. This provides a quick,
# throwaway cluster for testing Forklift's role in the migration process,
# without bogging the process down with licensing and other Enterprise-only
# bootstrap steps.
#
# Usage: hack/calico/calico-mock-cluster.sh
#   CALICO_MANIFESTS=<calico manifests base URL> CLUSTER_NAME=forklift-calico-mock
# Teardown: kind delete cluster --name forklift-calico-mock

# The Calico shipment the base installation comes from: a manifests directory
# of a Calico release (or hashrelease) that ships the projectcalico.org/v3
# CRD bundle (v3.32 and later).
CALICO_MANIFESTS="${CALICO_MANIFESTS:-https://raw.githubusercontent.com/projectcalico/calico/v3.32.2/manifests}"
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

# The v3 CRDs are installed here, the four Enterprise schemas are then layered on top and stay put.
echo "Installing the Calico projectcalico.org/v3 CRDs (api-server-less datastore)..."

# Install just CRDs.
curl -sL "${CALICO_MANIFESTS}/v3_projectcalico_org.yaml" \
  | python3 -c "import sys; print('\n---\n'.join(d for d in sys.stdin.read().split('\n---\n') if 'kind: CustomResourceDefinition' in d))" \
  | kubectl apply --server-side -f -

echo "Applying the Enterprise CRD schemas for every resource Forklift reads..."
# networks is Enterprise-only; the other three override the OSS schemas with
# the Enterprise ones (a superset carrying the fields Forklift's checks read).
kubectl apply --server-side --force-conflicts -f "${MOCK_DIR}/networks-crd.yaml"
kubectl apply --server-side --force-conflicts -f "${MOCK_DIR}/ippools-crd.yaml"
kubectl apply --server-side --force-conflicts -f "${MOCK_DIR}/felixconfigurations-crd.yaml"
kubectl apply --server-side --force-conflicts -f "${MOCK_DIR}/bgppeers-crd.yaml"

echo "Installing the tigera operator in api-server-less (v3-CRD) mode..."
# -manage-crds=false keeps the operator's hands off the CRDs applied above.
# With the v3 group already present at boot, the operator's API-mode discovery
# picks v3-CRD mode; CALICO_API_GROUP pins that decision explicitly.
curl -sL "${CALICO_MANIFESTS}/tigera-operator.yaml" \
  | sed 's/-manage-crds=true/-manage-crds=false/' \
  | kubectl create -f -
kubectl set env -n tigera-operator deployment/tigera-operator CALICO_API_GROUP=projectcalico.org/v3
kubectl wait --for=condition=Available --timeout=300s deployment -n tigera-operator tigera-operator
echo "Waiting for the operator API to be established..."
until kubectl get crd installations.operator.tigera.io >/dev/null 2>&1; do sleep 2; done
kubectl wait --for=condition=Established --timeout=120s crd/installations.operator.tigera.io

# Apply only the Installation from custom-resources: no APIServer CR — the
# projectcalico.org/v3 group is CRD-served.
curl -sL "${CALICO_MANIFESTS}/custom-resources.yaml" \
  | python3 -c "import sys; print('\n---\n'.join(d for d in sys.stdin.read().split('---') if 'kind: Installation' in d))" \
  | kubectl create -f -

echo "Waiting for nodes and Calico to become ready..."
kubectl wait --for=condition=Ready nodes --all --timeout=300s
kubectl -n calico-system rollout status daemonset/calico-node --timeout=300s
kubectl -n calico-system rollout status deployment/calico-kube-controllers --timeout=300s

# In api-server-less mode the cluster creates its own default
# FelixConfiguration in the v3 group; its presence proves the mode is real.
echo "Waiting for the native default FelixConfiguration..."
until kubectl get felixconfigurations.projectcalico.org default >/dev/null 2>&1; do sleep 2; done

echo "Applying sample resources..."
kubectl apply -f "${MOCK_DIR}/samples.yaml"

echo "Verifying Calico still works after the schema changes..."
kubectl -n calico-system rollout status daemonset/calico-node --timeout=120s
kubectl run mock-conncheck-a --image=busybox:1.36 --restart=Never -- sleep 300
kubectl run mock-conncheck-b --image=busybox:1.36 --restart=Never -- sleep 300
kubectl wait --for=condition=Ready pod/mock-conncheck-a pod/mock-conncheck-b --timeout=180s
B_IP="$(kubectl get pod mock-conncheck-b -o jsonpath='{.status.podIP}')"
kubectl exec mock-conncheck-a -- ping -c 3 -W 2 "$B_IP"
kubectl delete pod mock-conncheck-a mock-conncheck-b --wait=false

echo "Verifying the operator is healthy in api-server-less mode..."
for ts in calico ippools tiers; do
  kubectl wait --for=jsonpath='{.status.conditions[?(@.type=="Available")].status}'=True \
    "tigerastatus/${ts}" --timeout=120s
done

echo "Mock resources as stored:"
kubectl get networks.projectcalico.org mock-l2-net -o jsonpath='{.spec.l2Bridge.vlans[0]}'; echo
kubectl get ippools.projectcalico.org mock-vlan100-pool -o jsonpath='{.spec.allowedUses}'; echo
kubectl get felixconfigurations.projectcalico.org default -o name
# The Enterprise IPPool schema must survive the operator's own CRD management.
kubectl get crd ippools.projectcalico.org -o yaml | grep -q L2Workload \
  || { echo "STOP: Enterprise IPPool schema was overwritten"; exit 1; }

echo "API groups as Forklift's capability probe sees them:"
kubectl api-resources --api-group=projectcalico.org 2>/dev/null || true
# kubectl's discovery output above can lag a fresh CRD; the CRDs' Established
# condition is the authoritative check.
for r in networks ippools felixconfigurations bgppeers; do
  kubectl wait --for=condition=Established --timeout=60s "crd/${r}.projectcalico.org" >/dev/null \
    || { echo "MISSING ${r} in projectcalico.org"; exit 1; }
done
echo "Legacy storage group (absent in api-server-less mode):"
kubectl api-resources --api-group=crd.projectcalico.org 2>/dev/null || true

echo "Done. Cluster: ${CLUSTER_NAME} (kubectl context kind-${CLUSTER_NAME})"
