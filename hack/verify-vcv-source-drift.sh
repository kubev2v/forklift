#!/usr/bin/env bash

set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
vcv_root="${VCV_ROOT:?Set VCV_ROOT to the pinned virt-cluster-validate checkout}"

# shellcheck disable=SC1091
source "${root_dir}/operator/virt-validation-controller-source.env"

actual_ref="$(git -C "${vcv_root}" rev-parse HEAD)"
if [[ "${actual_ref}" != "${VCV_REF}" ]]; then
    echo "VCV_ROOT is at ${actual_ref}; expected ${VCV_REF}" >&2
    exit 1
fi

cmp "${vcv_root}/config/crd/bases/validation.kubevirt.io_virtualizationvalidations.yaml" \
    "${root_dir}/operator/config/crd/bases/validation.kubevirt.io_virtualizationvalidations.yaml"

source_role="$(mktemp)"
forklift_role="$(mktemp)"
trap 'rm -f "${source_role}" "${forklift_role}"' EXIT

sed '/^---/,$d' "${vcv_root}/config/rbac/role.yaml" >"${source_role}"
sed '/^---/,$d' "${root_dir}/operator/roles/forkliftcontroller/templates/virt-validation-controller/rbac.yml.j2" >"${forklift_role}"
cmp "${source_role}" "${forklift_role}"

echo "VirtualizationValidation API and controller RBAC match ${VCV_REPOSITORY}@${VCV_REF}."
