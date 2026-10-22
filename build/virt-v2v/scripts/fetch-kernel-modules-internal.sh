#!/usr/bin/env bash
# Prefetch kernel-modules-internal RPM matching the runtime image kernel (provides btrfs.ko).
set -euo pipefail

dest_dir="$(cd "$(dirname "$0")/.." && pwd)/prefetch"
mkdir -p "${dest_dir}"
rpm_basename="${KERNEL_MODULES_INTERNAL_RPM_BASENAME:-kernel-modules-internal-6.12.0-271.el10.x86_64.rpm}"
rpm_out="${dest_dir}/${rpm_basename}"
rpm_url="${KERNEL_MODULES_INTERNAL_RPM_URL:-https://kojihub.stream.centos.org/kojifiles/packages/kernel/6.12.0/271.el10/x86_64/${rpm_basename}}"

if [[ -s "${rpm_out}" ]]; then
    echo "Already present: ${rpm_out}"
    exit 0
fi

shopt -s nullglob
for existing in "${dest_dir}"/kernel-modules-internal-*.x86_64.rpm; do
    if [[ -s "${existing}" ]]; then
        echo "Already present: ${existing}"
        exit 0
    fi
done
shopt -u nullglob

if [[ -n "${KERNEL_MODULES_INTERNAL_RPM_PATH:-}" ]]; then
    cp -f "${KERNEL_MODULES_INTERNAL_RPM_PATH}" "${rpm_out}"
    echo "OK: copied to ${rpm_out}"
    exit 0
fi

curl_opts=(-fL --retry 5 --retry-delay 10 --retry-all-errors -A "forklift-fetch-kernel-modules-internal/1.0")
if [[ "${KERNEL_MODULES_INTERNAL_CURL_INSECURE:-0}" == "1" ]]; then
    curl_opts+=(-k)
fi
echo "Fetching ${rpm_url} -> ${rpm_out}"
curl "${curl_opts[@]}" -o "${rpm_out}" "${rpm_url}"
echo "OK: ${rpm_out}"
