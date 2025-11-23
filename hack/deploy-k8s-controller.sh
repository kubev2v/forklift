#!/bin/bash

# Deploy ForkliftController CR Script
# Usage: ./deploy-k8s-controller.sh [namespace]

NAMESPACE="${1:-konveyor-forklift}"
KUBECTL="${KUBECTL:-kubectl}"
TIMEOUT=300

echo "Waiting for Forklift operator to be ready in namespace: $NAMESPACE (timeout: ${TIMEOUT}s)..."

ELAPSED=0
while [[ $ELAPSED -lt $TIMEOUT ]]; do
    CSV_PHASE=$($KUBECTL get csv -n "$NAMESPACE" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "Unknown")
    if [[ "$CSV_PHASE" = "Succeeded" ]]; then
        echo "Operator is ready!"
        break
    fi
    printf "."
    sleep 5
    ELAPSED=$((ELAPSED + 5))
done

if [[ $ELAPSED -ge $TIMEOUT ]]; then
    echo "" >&2
    echo "Error: Operator did not become ready within ${TIMEOUT}s" >&2
    exit 1
fi

echo ""

if cat <<EOF | $KUBECTL apply -f -
apiVersion: forklift.konveyor.io/v1beta1
kind: ForkliftController
metadata:
  name: forklift-controller
  namespace: $NAMESPACE
spec: {}
EOF
then
    echo "ForkliftController deployed successfully in namespace: $NAMESPACE"
    echo ""
    echo "Use kubectl-mtv with port-forwarded inventory:"
    echo ""
    echo "   # In one terminal, forward the inventory service:"
    echo "   kubectl port-forward -n $NAMESPACE service/forklift-inventory 8443:8443"
    echo ""
    echo "   # In another terminal, set the inventory URL and use kubectl-mtv:"
    echo "   export INVENTORY_URL=https://localhost:8443"
    echo "   kubectl-mtv get provider"
else
    echo "Error: Failed to deploy ForkliftController in namespace: $NAMESPACE" >&2
    exit 1
fi
