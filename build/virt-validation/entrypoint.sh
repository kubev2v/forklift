#!/usr/bin/env bash
#
# Run VCV against a remote Provider and mirror complete CTRF snapshots into the
# controller-created ConfigMap. VCV owns report generation; this wrapper owns
# only Kubernetes transport on the hub cluster.
set -euo pipefail

readonly app_dir=/opt/app-root/src
readonly credentials_dir=/var/run/forklift-validation
readonly service_account_dir=/var/run/secrets/kubernetes.io/serviceaccount
readonly report_file="${VIRT_VALIDATE_REPORT_FILE:-/tmp/virt-validation/report.json}"
readonly max_report_bytes="${VIRT_VALIDATE_MAX_REPORT_BYTES:-921600}"

require_env() {
    local name="$1"
    if [[ -z "${!name:-}" ]]; then
        echo "ERROR: ${name} must be set" >&2
        exit 2
    fi
}

require_env VIRT_VALIDATE_URL
require_env VIRT_VALIDATE_RESULT_CONFIGMAP

if [[ ! -r "${credentials_dir}/token" ]]; then
    echo "ERROR: provider token is not mounted at ${credentials_dir}/token" >&2
    exit 2
fi
if [[ ! -r "${service_account_dir}/token" || ! -r "${service_account_dir}/ca.crt" ]]; then
    echo "ERROR: hub service-account credentials are not mounted" >&2
    exit 2
fi

configmap_namespace="${VIRT_VALIDATE_RESULT_CONFIGMAP_NAMESPACE:-}"
if [[ -z "${configmap_namespace}" ]]; then
    configmap_namespace="$(<"${service_account_dir}/namespace")"
fi
if [[ ! "${VIRT_VALIDATE_RESULT_CONFIGMAP}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] ||
   [[ ! "${configmap_namespace}" =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]]; then
    echo "ERROR: invalid result ConfigMap name or namespace" >&2
    exit 2
fi

tmp_dir="$(mktemp -d)"
remote_kubeconfig="${tmp_dir}/remote-kubeconfig"
hub_kubeconfig="${tmp_dir}/hub-kubeconfig"
patch_file="${tmp_dir}/configmap-patch.json"
cleanup() {
    rm -rf "${tmp_dir}"
}
trap cleanup EXIT

# download-tools.sh needs a working remote kubeconfig before VCV starts. VCV
# creates its own short-lived remote kubeconfig when it is invoked below.
export VIRT_VALIDATE_TOKEN="$(<"${credentials_dir}/token")"
if [[ -r "${credentials_dir}/insecureSkipVerify" ]]; then
    export VIRT_VALIDATE_INSECURE_SKIP_TLS="$(<"${credentials_dir}/insecureSkipVerify")"
fi
if [[ -r "${credentials_dir}/ca.crt" && -s "${credentials_dir}/ca.crt" ]]; then
    export VIRT_VALIDATE_CA_FILE="${credentials_dir}/ca.crt"
fi

python3 - "${remote_kubeconfig}" <<'PY'
import base64
import json
import os
import sys

cluster = {"server": os.environ["VIRT_VALIDATE_URL"]}
ca_file = os.environ.get("VIRT_VALIDATE_CA_FILE")
if os.environ.get("VIRT_VALIDATE_INSECURE_SKIP_TLS", "").lower() in ("1", "true", "yes"):
    cluster["insecure-skip-tls-verify"] = True
elif ca_file:
    with open(ca_file, "rb") as ca:
        cluster["certificate-authority-data"] = base64.b64encode(ca.read()).decode("ascii")
config = {
    "apiVersion": "v1", "kind": "Config",
    "clusters": [{"name": "target", "cluster": cluster}],
    "users": [{"name": "provider", "user": {"token": os.environ["VIRT_VALIDATE_TOKEN"]}}],
    "contexts": [{"name": "target", "context": {"cluster": "target", "user": "provider"}}],
    "current-context": "target",
}
with open(sys.argv[1], "w", encoding="utf-8") as output:
    json.dump(config, output)
os.chmod(sys.argv[1], 0o600)
PY

python3 - "${hub_kubeconfig}" "${service_account_dir}" <<'PY'
import base64
import json
import os
import sys

path, sa_dir = sys.argv[1:]
port = os.environ.get("KUBERNETES_SERVICE_PORT_HTTPS", "443")
with open(f"{sa_dir}/token", encoding="utf-8") as token:
    service_account_token = token.read().strip()
with open(f"{sa_dir}/ca.crt", "rb") as ca:
    ca_data = base64.b64encode(ca.read()).decode("ascii")
config = {
    "apiVersion": "v1", "kind": "Config",
    "clusters": [{"name": "hub", "cluster": {
        "server": f"https://{os.environ['KUBERNETES_SERVICE_HOST']}:{port}",
        "certificate-authority-data": ca_data,
    }}],
    "users": [{"name": "publisher", "user": {"token": service_account_token}}],
    "contexts": [{"name": "hub", "context": {"cluster": "hub", "user": "publisher"}}],
    "current-context": "hub",
}
with open(path, "w", encoding="utf-8") as output:
    json.dump(config, output)
os.chmod(path, 0o600)
PY

# Infer the conventional OpenShift apps domain from the provider API endpoint
# when the controller has not supplied an explicit override.
if [[ -z "${CLUSTER_DOMAIN:-}" ]]; then
    target_host="${VIRT_VALIDATE_URL#*://}"
    target_host="${target_host%%/*}"
    target_host="${target_host%%:*}"
    if [[ "${target_host}" == api.* ]]; then
        export CLUSTER_DOMAIN="${target_host#api.}"
    fi
fi

export KUBECONFIG="${remote_kubeconfig}"
"${app_dir}/bin/download-tools.sh"
unset KUBECONFIG

publish_report() {
    [[ -r "${report_file}" ]] || return 0
    python3 - "${report_file}" "${patch_file}" "${max_report_bytes}" <<'PY'
import json
import sys

report_path, patch_path, max_bytes = sys.argv[1], sys.argv[2], int(sys.argv[3])
with open(report_path, encoding="utf-8") as report_file:
    report = json.load(report_file)
# Pod logs retain verbose evidence. Keep the ConfigMap payload bounded and safe
# for status projection by omitting traces and limiting individual messages.
for test in report.get("results", {}).get("tests", []):
    test.pop("trace", None)
    if isinstance(test.get("message"), str):
        test["message"] = test["message"][:4096]
payload = json.dumps(report, separators=(",", ":"), ensure_ascii=False)
if len(payload.encode("utf-8")) > max_bytes:
    raise SystemExit("CTRF snapshot exceeds ConfigMap publication limit")
with open(patch_path, "w", encoding="utf-8") as patch_file:
    json.dump({"data": {"report.json": payload}}, patch_file, separators=(",", ":"))
PY
    oc --kubeconfig="${hub_kubeconfig}" -n "${configmap_namespace}" patch configmap \
        "${VIRT_VALIDATE_RESULT_CONFIGMAP}" --type=merge --patch "$(<"${patch_file}")" >/dev/null
}

mkdir -p "$(dirname "${report_file}")"
runner_args=(-o ctrf --report-file "${report_file}")
if [[ -n "${VIRT_VALIDATE_CHECKS:-}" ]]; then
    runner_args+=(--include "${VIRT_VALIDATE_CHECKS}")
fi

cd "${app_dir}"
"${app_dir}/virt-cluster-validate" "${runner_args[@]}" &
runner_pid=$!
last_digest=""
while kill -0 "${runner_pid}" 2>/dev/null; do
    if [[ -r "${report_file}" ]]; then
        digest="$(sha256sum "${report_file}" | awk '{print $1}')"
        if [[ "${digest}" != "${last_digest}" ]]; then
            publish_report
            last_digest="${digest}"
        fi
    fi
    sleep 1
done

runner_status=0
wait "${runner_pid}" || runner_status=$?
if [[ -r "${report_file}" ]]; then
    publish_report
fi
exit "${runner_status}"
