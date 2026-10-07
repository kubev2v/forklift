#!/usr/bin/env bash

set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
render_dir="$(mktemp -d)"
ansible_tmp="$(mktemp -d)"
trap 'rm -rf "${render_dir}" "${ansible_tmp}"' EXIT

require() {
    if ! grep -Fqx -- "$2" "$1"; then
        echo "Missing expected rendered value: $2 ($1)" >&2
        exit 1
    fi
}

contains() {
    if ! grep -Fq -- "$2" "$1"; then
        echo "Missing expected value: $2 ($1)" >&2
        exit 1
    fi
}

does_not_contain() {
    if grep -Fq -- "$2" "$1"; then
        echo "Unexpected value: $2 ($1)" >&2
        exit 1
    fi
}

command -v ansible-playbook >/dev/null || {
    echo "ansible-playbook is required to render packaging assets" >&2
    exit 1
}

cd "${root_dir}"
ANSIBLE_LOCAL_TEMP="${ansible_tmp}" ANSIBLE_REMOTE_TEMP="${ansible_tmp}" \
    ansible-playbook -i localhost, -c local operator/tests/vcv-packaging-render.yml \
    -e "output_dir=${render_dir}" >/dev/null

deployment="${render_dir}/deployment.yaml"
rbac="${render_dir}/rbac.yaml"
network_policy="${render_dir}/networkpolicy.yaml"
service_monitor="${render_dir}/servicemonitor.yaml"

require "${deployment}" "  namespace: openshift-mtv"
require "${deployment}" "        image: \"quay.io/example/virt-validation-controller@sha256:controller\""
require "${deployment}" "        - --validator-image=quay.io/example/virt-cluster-validate@sha256:validator"
require "${deployment}" "        - --leader-election-namespace=openshift-mtv"
require "${rbac}" "  namespace: openshift-mtv"
require "${rbac}" "  name: virt-validation-controller"
contains "${rbac}" "  - virtualizationvalidations/status"
contains "${service_monitor}" "  scrapeClass: tls-client-certificate-auth"
require "${service_monitor}" "      serverName: virt-validation-controller-metrics.openshift-mtv.svc"

if [[ "$(grep -Fc '      component: virt-validation-controller' "${network_policy}")" -ne 4 ]]; then
    echo "Each VirtualizationValidation NetworkPolicy must target only its controller pods" >&2
    exit 1
fi

contains operator/config/crd/kustomization.yaml "- bases/validation.kubevirt.io_virtualizationvalidations.yaml"
contains operator/config/manager/manager.yaml "        - name: VIRT_VALIDATION_CONTROLLER_IMAGE"
contains operator/config/manager/manager.yaml "        - name: VIRT_CLUSTER_VALIDATE_IMAGE"
contains operator/related_images.yaml "  - name: virt-validation-controller"
contains operator/related_images.yaml "  - name: virt-cluster-validate"
contains operator/roles/forkliftcontroller/tasks/main.yml "Require virtualization validations to be removed before finalizing"
if [[ "$(grep -Fc "when: not k8s_cluster|bool and 'validation.kubevirt.io' in api_groups" operator/roles/forkliftcontroller/tasks/main.yml)" -ne 2 ]]; then
    echo "VirtualizationValidation finalization checks must require the validation API group" >&2
    exit 1
fi
contains operator/config/rbac/role.yaml "  - validation.kubevirt.io"
contains operator/config/rbac/role.yaml "  - virtualizationvalidations/status"
does_not_contain operator/config/rbac/role.yaml "  - escalate"
does_not_contain operator/config/rbac/role.yaml "  - bind"
contains operator/streams/upstream/forklift-operator.clusterserviceversion.yaml '      image: "${VIRT_VALIDATION_CONTROLLER_IMAGE}"'
contains operator/streams/upstream/forklift-operator.clusterserviceversion.yaml '      image: "${VIRT_CLUSTER_VALIDATE_IMAGE}"'

kubectl kustomize operator/config/default >/dev/null
echo "VirtualizationValidation packaging assets are valid."
