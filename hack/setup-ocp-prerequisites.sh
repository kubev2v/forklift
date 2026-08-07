#!/bin/bash
set -e

# Usage: ./setup-ocp-prerequisites.sh [namespace]
# Installs the KubeVirt HyperConverged operator on OCP/OKD via OLM.
#
# Environment variables:
#   KUBECTL          - CLI binary (default: oc)
#   HCO_PACKAGE      - OLM package name (default: community-kubevirt-hyperconverged)
#   HCO_SOURCE       - CatalogSource name (default: community-operators)
#   HCO_CHANNEL      - Subscription channel (default: stable)
#   TIMEOUT          - Max wait time in seconds (default: 900)

NAMESPACE="${1:-kubevirt-hyperconverged}"
KUBECTL="${KUBECTL:-oc}"
HCO_PACKAGE="${HCO_PACKAGE:-community-kubevirt-hyperconverged}"
HCO_SOURCE="${HCO_SOURCE:-community-operators}"
HCO_CHANNEL="${HCO_CHANNEL:-stable}"
TIMEOUT="${TIMEOUT:-900}"

echo "Checking cluster connectivity..."
$KUBECTL cluster-info > /dev/null 2>&1 || { echo "Error: No OpenShift cluster found" >&2; exit 1; }

# Check that the marketplace is available
echo "Checking openshift-marketplace..."
$KUBECTL get catalogsource "$HCO_SOURCE" -n openshift-marketplace > /dev/null 2>&1 || {
    echo "Error: CatalogSource '${HCO_SOURCE}' not found in openshift-marketplace" >&2
    echo "Is the marketplace operator enabled?" >&2
    exit 1
}

# Skip if HyperConverged is already available
if $KUBECTL get HyperConverged kubevirt-hyperconverged -n "$NAMESPACE" \
    --no-headers 2>/dev/null | grep -q .; then
    echo "HyperConverged CR already exists in namespace: ${NAMESPACE}, skipping install."
    $KUBECTL wait HyperConverged kubevirt-hyperconverged -n "$NAMESPACE" \
        --for=condition=Available --timeout=60s 2>/dev/null && {
        echo "HyperConverged is Available. Nothing to do."
        exit 0
    }
    echo "Warning: HyperConverged exists but is not yet Available, continuing to wait..."
else
    # Install HyperConverged Operator
    echo "Installing KubeVirt HyperConverged Operator (${HCO_PACKAGE}) in namespace: ${NAMESPACE}..."
    cat << EOF | $KUBECTL apply -f -
---
apiVersion: v1
kind: Namespace
metadata:
  name: ${NAMESPACE}
  labels:
    kubernetes.io/metadata.name: ${NAMESPACE}
    openshift.io/cluster-monitoring: "true"
    pod-security.kubernetes.io/audit: privileged
    pod-security.kubernetes.io/audit-version: v1.24
    pod-security.kubernetes.io/warn: privileged
    pod-security.kubernetes.io/warn-version: v1.24
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: kubevirt-hyperconverged-group
  namespace: ${NAMESPACE}
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: ${HCO_PACKAGE}
  namespace: ${NAMESPACE}
spec:
  source: ${HCO_SOURCE}
  sourceNamespace: openshift-marketplace
  name: ${HCO_PACKAGE}
  channel: "${HCO_CHANNEL}"
  installPlanApproval: Automatic
EOF

    echo "Waiting for HCO operator to be ready..."
    ELAPSED=0
    while [[ $ELAPSED -lt $TIMEOUT ]]; do
        if $KUBECTL get pods -n "$NAMESPACE" -l name=hyperconverged-cluster-webhook --no-headers 2>/dev/null | grep -q .; then
            if $KUBECTL wait pods -n "$NAMESPACE" -l name=hyperconverged-cluster-webhook \
                --for=jsonpath='{.status.containerStatuses[0].ready}'=true --timeout=10s 2>/dev/null; then
                echo "HCO webhook is ready!"
                break
            fi
        fi
        printf "."
        sleep 10
        ELAPSED=$((ELAPSED + 10))
    done

    if [[ $ELAPSED -ge $TIMEOUT ]]; then
        echo "" >&2
        echo "Error: HCO operator did not become ready within ${TIMEOUT}s" >&2
        exit 1
    fi

    # Create HyperConverged CR
    echo "Creating HyperConverged CR..."
    ELAPSED=0
    LAST_APPLY_ERR=""
    until LAST_APPLY_ERR=$(cat << EOF | $KUBECTL apply -f - 2>&1)
apiVersion: hco.kubevirt.io/v1beta1
kind: HyperConverged
metadata:
  name: kubevirt-hyperconverged
  namespace: ${NAMESPACE}
spec:
  enableCommonBootImageImport: false
EOF
    do
        if [[ $ELAPSED -ge $TIMEOUT ]]; then
            echo "" >&2
            echo "Error: Failed to create HyperConverged CR within ${TIMEOUT}s" >&2
            echo "Last apply error:" >&2
            echo "${LAST_APPLY_ERR}" >&2
            echo "" >&2
            echo "OLM diagnostics:" >&2
            $KUBECTL get subscription -n "$NAMESPACE" -o wide >&2 || true
            $KUBECTL get csv -n "$NAMESPACE" -o wide >&2 || true
            $KUBECTL get installplan -n "$NAMESPACE" -o wide >&2 || true
            exit 1
        fi
        echo "Retrying HyperConverged CR creation (elapsed ${ELAPSED}s/${TIMEOUT}s)..."
        sleep 20
        ELAPSED=$((ELAPSED + 20))
    done
    echo "${LAST_APPLY_ERR}"
fi

echo "Waiting for HyperConverged to become Available (this may take several minutes)..."
$KUBECTL wait HyperConverged kubevirt-hyperconverged -n "$NAMESPACE" \
    --for=condition=Available --timeout=${TIMEOUT}s

echo ""
echo "Installed components:"
echo "  KubeVirt HCO: ${HCO_PACKAGE} (${HCO_CHANNEL} channel)"
echo "  Source: ${HCO_SOURCE}"
echo "  Namespace: ${NAMESPACE}"
echo ""
echo "We are ready to deploy forklift"
