#!/usr/bin/env bash

set -e

if [[ -z "${REGISTRY}" || -z "${REGISTRY_TAG}" || -z  "${REGISTRY_ORG}" ]]; then
    echo "Please set all REGISTRY, REGISTRY_TAG and REGISTRY_ORG environment variables!"
    exit 1
fi

CONTROLLER_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-controller:${REGISTRY_TAG}
OPERATOR_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-operator:${REGISTRY_TAG}
MUST_GATHER_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-must-gather:${REGISTRY_TAG}
UI_PLUGIN_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-console-plugin:${REGISTRY_TAG}
VALIDATION_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-validation:${REGISTRY_TAG}
VIRT_V2V_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-virt-v2v:${REGISTRY_TAG}
VIRT_V2V_WARM_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-virt-v2v-warm:${REGISTRY_TAG}
API_IMAGE=${REGISTRY}/${REGISTRY_ORG}/forklift-api:${REGISTRY_TAG}
POPULATOR_CONTROLLER_IMAGE=${REGISTRY}/${REGISTRY_ORG}/populator-controller:${REGISTRY_TAG}
OVA_PROVIDER_SERVER=${REGISTRY}/${REGISTRY_ORG}/forklift-ova-provider-server:${REGISTRY_TAG}

bazel run push-forklift-api
bazel run push-ovirt-populator
bazel run push-openstack-populator
bazel run --package_path=virt-v2v/cold push-forklift-virt-v2v
bazel run --package_path=virt-v2v/warm push-forklift-virt-v2v-warm
bazel run push-populator-controller
bazel run push-forklift-controller
bazel run push-forklift-validation
bazel run push-ova-provider-server
bazel run push-forklift-operator
bazel run push-forklift-operator-bundle \
    --action_env OPERATOR_IMAGE=${OPERATOR_IMAGE} \
    --action_env MUST_GATHER_IMAGE=${MUST_GATHER_IMAGE} \
    --action_env UI_PLUGIN_IMAGE=${UI_PLUGIN_IMAGE} \
    --action_env VALIDATION_IMAGE=${VALIDATION_IMAGE} \
    --action_env VIRT_V2V_IMAGE=${VIRT_V2V_IMAGE} \
    --action_env VIRT_V2V_WARM_IMAGE=${VIRT_V2V_WARM_IMAGE} \
    --action_env CONTROLLER_IMAGE=${CONTROLLER_IMAGE} \
    --action_env API_IMAGE=${API_IMAGE} \
    --action_env POPULATOR_CONTROLLER_IMAGE=${POPULATOR_CONTROLLER_IMAGE} \
    --action_env OVA_PROVIDER_SERVER=${OVA_PROVIDER_SERVER=}
bazel run push-forklift-operator-index \
    --action_env REGISTRY=${REGISTRY} \
    --action_env REGISTRY_TAG=${REGISTRY_TAG} \
    --action_env REGISTRY_ORG=${REGISTRY_ORG}
