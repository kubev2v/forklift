#!/bin/bash
# Run podman from the toehold appliance installroot (same pattern as vmtoolsd).
set -euo pipefail
root=/opt/toehold/appliance-root
conf="${root}/etc/containers"
export PATH="${root}/usr/bin:${root}/usr/sbin:${PATH}"
if [[ -x "${root}/usr/bin/conmon" ]]; then
  export CONMON_BINARY="${root}/usr/bin/conmon"
fi
exec env \
  LD_LIBRARY_PATH="${root}/usr/lib64:${root}/usr/lib${LD_LIBRARY_PATH:+:${LD_LIBRARY_PATH}}" \
  CONTAINERS_STORAGE_CONF="${conf}/storage.conf" \
  CONTAINERS_CONF="${conf}/containers.conf" \
  CONTAINERS_POLICY_JSON="${conf}/policy.json" \
  "${root}/usr/bin/podman" --storage-driver vfs "$@"
