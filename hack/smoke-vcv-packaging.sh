#!/usr/bin/env bash

set -euo pipefail

namespace="${1:-openshift-mtv}"
timeout="${TIMEOUT:-180s}"
oc_bin="${OC:-oc}"
controller="virt-validation-controller"

"${oc_bin}" get crd virtualizationvalidations.validation.kubevirt.io >/dev/null
"${oc_bin}" rollout status "deployment/${controller}" -n "${namespace}" --timeout="${timeout}"
"${oc_bin}" wait --for=condition=Ready "pod" \
    -l app=forklift-controller,component=virt-validation-controller \
    -n "${namespace}" --timeout="${timeout}"

"${oc_bin}" get "service/${controller}-metrics" -n "${namespace}" >/dev/null
"${oc_bin}" get "secret/${controller}-metrics-tls" -n "${namespace}" >/dev/null
"${oc_bin}" get "servicemonitor/${controller}" -n "${namespace}" >/dev/null
"${oc_bin}" get clusterrole "${controller}" >/dev/null
"${oc_bin}" get clusterrolebinding "${controller}" >/dev/null

for policy in default-deny allow-api allow-dns allow-metrics; do
    "${oc_bin}" get "networkpolicy/${controller}-${policy}" -n "${namespace}" >/dev/null
done

echo "VirtualizationValidation controller packaging smoke test passed in ${namespace}."
