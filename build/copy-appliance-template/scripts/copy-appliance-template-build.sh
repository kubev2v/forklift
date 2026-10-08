#!/usr/bin/env bash
set -euo pipefail

export LIBGUESTFS_BACKEND=direct

WORK=/work
OVERLAY="${WORK}/overlay.qcow2"
VMDK="${WORK}/disk-0.vmdk"
TARBALL=/usr/share/copy-appliance-template/appliance-root.tar.gz

shopt -s nullglob
disks=(/disk/disk/*.qcow2)
if (( ${#disks[@]} != 1 )); then
  echo "copy-appliance-template-build: expected one qcow2 under /disk/disk, found ${#disks[@]}" >&2
  exit 1
fi

qemu-img create -f qcow2 -b "${disks[0]}" -F qcow2 "${OVERLAY}"

args=(-a "${OVERLAY}")
if [[ -n "${COPY_APPLIANCE_TEMPLATE_SSH_PUBLIC_KEY_FILE:-}" && -f "${COPY_APPLIANCE_TEMPLATE_SSH_PUBLIC_KEY_FILE}" ]]; then
  echo "copy-appliance-template-build: installing SSH public key from ${COPY_APPLIANCE_TEMPLATE_SSH_PUBLIC_KEY_FILE}"
  args+=(--ssh-inject "root:file:${COPY_APPLIANCE_TEMPLATE_SSH_PUBLIC_KEY_FILE}")
elif [[ -n "${COPY_APPLIANCE_TEMPLATE_SSH_PUBLIC_KEY_FILE:-}" ]]; then
  echo "copy-appliance-template-build: SSH public key file missing at ${COPY_APPLIANCE_TEMPLATE_SSH_PUBLIC_KEY_FILE}" >&2
fi
args+=(--copy-in "${TARBALL}:/root/" --run /usr/local/bin/extract-appliance-root)

virt-customize -vvv -x "${args[@]}"

qemu-img convert -f qcow2 -O vmdk -o subformat=streamOptimized "${OVERLAY}" "${VMDK}"
export COPY_APPLIANCE_TEMPLATE_VMDK_PATH="${VMDK}"
exec copy-appliance-template-uploader
