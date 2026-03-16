#!/usr/bin/env bash

# Forklift Cluster Scrub Script
# Discovers and removes all Forklift/MTV resources from the cluster.
# Usage: ./hack/cleanup-forklift.sh [--dry-run] [--namespace <ns>]

set -euo pipefail

KUBECTL="${KUBECTL:-kubectl}"
DRY_RUN=false
FORCE_NS=""
FOUND_ITEMS=0
CLEANED_ITEMS=0

readonly RE_FORKLIFT_CRD='forklift\.konveyor\.io'
readonly RE_FORKLIFT_CDI_CRD='forklift\.cdi\.kubevirt\.io'
readonly RE_FORKLIFT_OR_KONVEYOR='forklift|konveyor'
readonly RE_FORKLIFT='forklift'

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Remove all Forklift (MTV) resources from the cluster.

Options:
  --dry-run              Show what would be deleted without making changes
  --namespace <ns>       Target a specific namespace (default: auto-detect)
  -h, --help             Show this help message

The script will:
  1. Auto-detect the Forklift operator namespace
  2. Delete ForkliftController CRs (clearing finalizers if needed)
  3. Delete OLM resources (Subscription, CSV, InstallPlans)
  4. Delete the operator namespace
  5. Delete cluster-scoped resources (CRDs, ClusterRoles, ClusterRoleBindings, ConsolePlugins)
  6. Print a summary report
EOF
    exit 0
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)   DRY_RUN=true; shift ;;
        --namespace) FORCE_NS="$2"; shift 2 ;;
        -h|--help)   usage ;;
        *)           echo "Unknown option: $1" >&2; usage ;;
    esac
done

header() { local msg="$1"; echo "" ; echo "=== $msg ===" ; return 0; }
found()  { FOUND_ITEMS=$((FOUND_ITEMS + 1)); return 0; }
cleaned(){ CLEANED_ITEMS=$((CLEANED_ITEMS + 1)); return 0; }

run_or_dry() {
    local cmd="$*"
    if $DRY_RUN; then
        echo "  [dry-run] $cmd"
    else
        eval "$cmd" 2>&1 | sed 's/^/  /'
    fi
    return 0
}

collect_lines() {
    local _result=()
    while IFS= read -r line; do
        [[ -n "$line" ]] && _result+=("$line")
    done
    printf '%s\n' "${_result[@]}" 2>/dev/null | sort -u
    return 0
}

# --- Detect namespace ---
header "Detecting Forklift namespace"

NAMESPACES=()
if [[ -n "$FORCE_NS" ]]; then
    NAMESPACES=("$FORCE_NS")
    echo "  Using provided namespace: $FORCE_NS"
else
    while IFS= read -r ns; do
        [[ -n "$ns" ]] && NAMESPACES+=("$ns")
    done < <(
        $KUBECTL get deployments --all-namespaces \
            -l app=forklift,name=controller-manager \
            -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null \
        | sort -u
    )

    if [[ ${#NAMESPACES[@]} -eq 0 ]]; then
        while IFS= read -r ns; do
            [[ -n "$ns" ]] && NAMESPACES+=("$ns")
        done < <(
            $KUBECTL get pods --all-namespaces \
                -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null \
            | sort -u \
            | while IFS= read -r ns; do
                if $KUBECTL get pods -n "$ns" -o name 2>/dev/null | grep -qiE "$RE_FORKLIFT_OR_KONVEYOR"; then
                    echo "$ns"
                fi
              done
        )
    fi

    if [[ ${#NAMESPACES[@]} -eq 0 ]]; then
        echo "  No Forklift namespaces detected."
    else
        echo "  Detected namespace(s): ${NAMESPACES[*]}"
    fi
fi

# --- Collect user-resource namespaces (any ns with Forklift CRs) ---
header "Scanning for Forklift custom resources in all namespaces"

USER_NAMESPACES=()
for crd_name in $($KUBECTL get crds -o name 2>/dev/null | grep "$RE_FORKLIFT_CRD" | sed 's|customresourcedefinition.apiextensions.k8s.io/||'); do
    while IFS= read -r ns; do
        [[ -n "$ns" ]] && USER_NAMESPACES+=("$ns")
    done < <($KUBECTL get "$crd_name" --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null | sort -u)
done

# Deduplicate
SEEN_NS=""
UNIQUE_USER_NS=()
for ns in "${USER_NAMESPACES[@]+"${USER_NAMESPACES[@]}"}"; do
    if [[ -n "$ns" ]] && ! echo "$SEEN_NS" | grep -qx "$ns"; then
        UNIQUE_USER_NS+=("$ns")
        SEEN_NS="$SEEN_NS
$ns"
    fi
done
USER_NAMESPACES=("${UNIQUE_USER_NS[@]+"${UNIQUE_USER_NS[@]}"}")

if [[ ${#USER_NAMESPACES[@]} -gt 0 ]]; then
    echo "  Found Forklift CRs in namespace(s): ${USER_NAMESPACES[*]}"
else
    echo "  No Forklift custom resources found."
fi

# --- Build combined namespace list ---
SEEN_NS=""
ALL_NS=()
for ns in "${NAMESPACES[@]+"${NAMESPACES[@]}"}" "${USER_NAMESPACES[@]+"${USER_NAMESPACES[@]}"}"; do
    if [[ -n "$ns" ]] && ! echo "$SEEN_NS" | grep -qx "$ns"; then
        ALL_NS+=("$ns")
        SEEN_NS="$SEEN_NS
$ns"
    fi
done

# --- Clear finalizers on Forklift CRs ---
header "Clearing finalizers on Forklift custom resources"

for crd_name in $($KUBECTL get crds -o name 2>/dev/null | grep "$RE_FORKLIFT_CRD" | sed 's|customresourcedefinition.apiextensions.k8s.io/||'); do
    for ns in "${ALL_NS[@]+"${ALL_NS[@]}"}"; do
        [[ -z "$ns" ]] && continue
        for item in $($KUBECTL get "$crd_name" -n "$ns" -o name 2>/dev/null); do
            echo "  Clearing finalizers: $item in $ns"
            found
            run_or_dry "$KUBECTL patch '$item' -n '$ns' --type=merge -p '{\"metadata\":{\"finalizers\":[]}}'"
            cleaned
        done
    done
done

# --- Delete OLM resources across all namespaces ---
header "Deleting Forklift OLM resources (scanning all namespaces)"

OLM_NS_SEEN=""
while IFS= read -r ns; do
    [[ -z "$ns" ]] && continue
    echo "$OLM_NS_SEEN" | grep -qx "$ns" && continue
    OLM_NS_SEEN="$OLM_NS_SEEN
$ns"
    echo "  Namespace: $ns"

    for sub in $($KUBECTL get subscriptions.operators.coreos.com -n "$ns" -o name 2>/dev/null | grep -iE "$RE_FORKLIFT_OR_KONVEYOR"); do
        echo "    Subscription: $sub"
        found
        run_or_dry "$KUBECTL delete '$sub' -n '$ns' --timeout=30s"
        cleaned
    done

    for csv in $($KUBECTL get csv -n "$ns" -o name 2>/dev/null | grep -iE "$RE_FORKLIFT_OR_KONVEYOR"); do
        echo "    CSV: $csv"
        found
        run_or_dry "$KUBECTL delete '$csv' -n '$ns' --timeout=30s"
        cleaned
    done

    for ip in $($KUBECTL get installplan -n "$ns" -o jsonpath='{range .items[*]}{.metadata.name}{" "}{range .spec.clusterServiceVersionNames[*]}{.}{" "}{end}{"\n"}{end}' 2>/dev/null | grep -iE "$RE_FORKLIFT_OR_KONVEYOR" | awk '{print $1}'); do
        echo "    InstallPlan: $ip"
        found
        run_or_dry "$KUBECTL delete installplan '$ip' -n '$ns' --timeout=30s"
        cleaned
    done
done < <(
    {
        $KUBECTL get subscriptions.operators.coreos.com --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null | sort -u | while IFS= read -r ns; do
            if $KUBECTL get subscriptions.operators.coreos.com -n "$ns" -o name 2>/dev/null | grep -qiE "$RE_FORKLIFT_OR_KONVEYOR"; then
                echo "$ns"
            fi
        done
        $KUBECTL get csv --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}{"\n"}{end}' 2>/dev/null | sort -u | while IFS= read -r ns; do
            if $KUBECTL get csv -n "$ns" -o name 2>/dev/null | grep -qiE "$RE_FORKLIFT_OR_KONVEYOR"; then
                echo "$ns"
            fi
        done
    } | sort -u
)

header "Deleting Forklift OperatorGroups in operator namespaces"

for ns in "${NAMESPACES[@]+"${NAMESPACES[@]}"}"; do
    [[ -z "$ns" ]] && continue
    for og in $($KUBECTL get operatorgroup -n "$ns" -o name 2>/dev/null); do
        echo "  OperatorGroup: $og in $ns"
        found
        run_or_dry "$KUBECTL delete '$og' -n '$ns' --timeout=30s"
        cleaned
    done
done

header "Deleting Forklift CatalogSources (scanning all namespaces)"

while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    cs_ns=$(echo "$line" | awk '{print $1}')
    cs_name=$(echo "$line" | awk '{print $2}')
    echo "  catalogsource/$cs_name in $cs_ns"
    found
    run_or_dry "$KUBECTL delete catalogsource '$cs_name' -n '$cs_ns' --timeout=30s"
    cleaned
done < <(
    $KUBECTL get catalogsource --all-namespaces -o jsonpath='{range .items[*]}{.metadata.namespace}{" "}{.metadata.name}{"\n"}{end}' 2>/dev/null \
    | grep -iE "$RE_FORKLIFT_OR_KONVEYOR"
)

# --- Delete namespaces ---
header "Deleting namespaces"

for ns in "${ALL_NS[@]+"${ALL_NS[@]}"}"; do
    [[ -z "$ns" ]] && continue
    status=$($KUBECTL get ns "$ns" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
    if [[ "$status" == "NotFound" ]]; then
        echo "  Namespace $ns: already gone"
        continue
    fi
    echo "  Namespace $ns (status: $status)"
    found
    run_or_dry "$KUBECTL delete namespace '$ns' --timeout=120s --wait=false"
    cleaned
done

# --- Delete CRDs ---
header "Deleting Forklift CRDs"

for crd in $($KUBECTL get crds -o name 2>/dev/null | grep "$RE_FORKLIFT_CRD"); do
    echo "  $crd"
    found
    run_or_dry "$KUBECTL delete '$crd' --timeout=30s"
    cleaned
done

CDI_CRDS=$($KUBECTL get crds -o name 2>/dev/null | grep "$RE_FORKLIFT_CDI_CRD" || true)
if [[ -n "$CDI_CRDS" ]]; then
    echo "  (skipping forklift.cdi.kubevirt.io CRDs -- managed by CDI, not Forklift)"
fi

# --- Delete ClusterRoles ---
header "Deleting Forklift ClusterRoles"

for cr in $($KUBECTL get clusterroles -o name 2>/dev/null | grep "$RE_FORKLIFT"); do
    echo "  $cr"
    found
    run_or_dry "$KUBECTL delete '$cr'"
    cleaned
done

# --- Delete ClusterRoleBindings ---
header "Deleting Forklift ClusterRoleBindings"

for crb in $($KUBECTL get clusterrolebindings -o name 2>/dev/null | grep "$RE_FORKLIFT"); do
    echo "  $crb"
    found
    run_or_dry "$KUBECTL delete '$crb'"
    cleaned
done

# --- Delete ConsolePlugins ---
header "Deleting Forklift ConsolePlugins"

for cp in $($KUBECTL get consoleplugins -o name 2>/dev/null | grep "$RE_FORKLIFT"); do
    echo "  $cp"
    found
    run_or_dry "$KUBECTL delete '$cp'"
    cleaned
done

# --- Wait for namespaces to be fully gone ---
if ! $DRY_RUN && [[ ${#ALL_NS[@]} -gt 0 ]]; then
    header "Waiting for namespace deletion"
    for ns in "${ALL_NS[@]+"${ALL_NS[@]}"}"; do
        [[ -z "$ns" ]] && continue
        ELAPSED=0
        TIMEOUT=120
        while [[ $ELAPSED -lt $TIMEOUT ]]; do
            if ! $KUBECTL get ns "$ns" &>/dev/null; then
                echo "  Namespace $ns: deleted"
                break
            fi
            printf "."
            sleep 5
            ELAPSED=$((ELAPSED + 5))
        done
        if [[ $ELAPSED -ge $TIMEOUT ]]; then
            echo ""
            echo "  WARNING: Namespace $ns still Terminating after ${TIMEOUT}s."
            echo "  You may need to manually clear remaining finalizers."
        fi
    done
fi

# --- Final report ---
header "CLEANUP REPORT"

REMAINING_CRDS=$($KUBECTL get crds -o name 2>/dev/null | grep -c "$RE_FORKLIFT" 2>/dev/null || true)
REMAINING_CRDS="${REMAINING_CRDS:-0}"
REMAINING_CR=$($KUBECTL get clusterroles -o name 2>/dev/null | grep -c "$RE_FORKLIFT" 2>/dev/null || true)
REMAINING_CR="${REMAINING_CR:-0}"
REMAINING_NS=0
for ns in "${ALL_NS[@]+"${ALL_NS[@]}"}"; do
    [[ -z "$ns" ]] && continue
    if $KUBECTL get ns "$ns" &>/dev/null; then
        REMAINING_NS=$((REMAINING_NS + 1))
    fi
done

if $DRY_RUN; then
    echo "  Mode:     DRY RUN (no changes made)"
else
    echo "  Mode:     LIVE"
fi
echo "  Found:    $FOUND_ITEMS resource(s)"
echo "  Cleaned:  $CLEANED_ITEMS resource(s)"
echo ""
echo "  Remaining cluster state:"
echo "    CRDs:                $REMAINING_CRDS (forklift.cdi.kubevirt.io CRDs are CDI-managed)"
echo "    ClusterRoles:        $REMAINING_CR"
echo "    Forklift namespaces: $REMAINING_NS"

if [[ "$REMAINING_CRDS" -le 2 && "$REMAINING_CR" -eq 0 && "$REMAINING_NS" -eq 0 ]]; then
    echo ""
    echo "  Cluster is CLEAN."
else
    echo ""
    echo "  WARNING: Some resources remain. Re-run or inspect manually."
fi
echo ""
