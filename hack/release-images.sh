#!/usr/bin/env bash

set -e 

if [[ -z "${REGISTRY}" || -z "${REGISTRY_TAG}" || -z  "${REGISTRY_ACCOUNT}" ]]; then
    echo "Please set all REGISTRY, REGISTRY_TAG and REGISTRY_ACCOUNT environment variables!" 
    exit 1
fi

CONTROLLER_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-controller:${REGISTRY_TAG}
OPERATOR_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-operator:${REGISTRY_TAG}
MUST_GATHER_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-must-gather:${REGISTRY_TAG}
MUST_GATHER_API_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-must-gather-api:${REGISTRY_TAG}
UI_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-ui:${REGISTRY_TAG}
UI_PLUGIN_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-console-plugin:${REGISTRY_TAG}
VALIDATION_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-validation:${REGISTRY_TAG}
VIRT_V2V_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-virt-v2v:${REGISTRY_TAG}
API_IMAGE=${REGISTRY}/${REGISTRY_ACCOUNT}/forklift-api:${REGISTRY_TAG}

export USE_BAZEL_VERSION=5.4.0

bazel run push-forklift-api
bazel run push-forklift-virt-v2v
bazel run push-forklift-controller
bazel run push-forklift-validation
bazel run push-forklift-operator
bazel run push-forklift-operator-bundle \
    --action_env OPERATOR_IMAGE=${OPERATOR_IMAGE} \
    --action_env MUST_GATHER_IMAGE=${MUST_GATHER_IMAGE} \
    --action_env MUST_GATHER_API_IMAGE=${MUST_GATHER_API_IMAGE} \
    --action_env UI_IMAGE=${UI_IMAGE} \
    --action_env UI_PLUGIN_IMAGE=${UI_PLUGIN_IMAGE} \
    --action_env VALIDATION_IMAGE=${VALIDATION_IMAGE} \
    --action_env VIRT_V2V_IMAGE=${VIRT_V2V_IMAGE} \
    --action_env CONTROLLER_IMAGE=${CONTROLLER_IMAGE} \
    --action_env API_IMAGE=${API_IMAGE}
bazel run push-forklift-operator-index \
    --action_env REGISTRY=${REGISTRY} \
    --action_env REGISTRY_TAG=${REGISTRY_TAG} \
    --action_env REGISTRY_ACCOUNT=${REGISTRY_ACCOUNT}
