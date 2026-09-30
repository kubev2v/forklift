#!/usr/bin/env bash
# Prefetch virtio-win RPM for image build (staged as /usr/local/virtio-win-legacy.iso).
set -euo pipefail

dest_dir="$(cd "$(dirname "$0")/.." && pwd)/prefetch"
mkdir -p "${dest_dir}"
rpm_basename="${VIRTIO_WIN_LEGACY_RPM_BASENAME:-virtio-win-0.1.160-1.noarch.rpm}"
rpm_out="${dest_dir}/${rpm_basename}"
rpm_url="${VIRTIO_WIN_LEGACY_RPM_URL:-https://fedorapeople.org/groups/virt/virtio-win/repo/rpms/${rpm_basename}}"

if [[ -s "${rpm_out}" ]]; then
    echo "Already present: ${rpm_out}"
    exit 0
fi

shopt -s nullglob
for existing in "${dest_dir}"/virtio-win-*.noarch.rpm; do
    if [[ -s "${existing}" ]]; then
        echo "Already present: ${existing}"
        exit 0
    fi
done
shopt -u nullglob

if [[ -n "${VIRTIO_WIN_LEGACY_RPM_PATH:-}" ]]; then
    cp -f "${VIRTIO_WIN_LEGACY_RPM_PATH}" "${rpm_out}"
    echo "OK: copied to ${rpm_out}"
    exit 0
fi

curl_opts=(-fL --retry 5 --retry-delay 10 --retry-all-errors -A "forklift-fetch-virtio-win-legacy/1.0")
if [[ "${VIRTIO_WIN_LEGACY_CURL_INSECURE:-0}" == "1" ]]; then
    curl_opts+=(-k)
fi
echo "Fetching ${rpm_url} -> ${rpm_out}"
curl "${curl_opts[@]}" -o "${rpm_out}" "${rpm_url}"
echo "OK: ${rpm_out}"
