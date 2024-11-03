#!/bin/bash -e

declare -A images

# The image refrences below will get updated by Konflux everytime there is a new image.

images[API_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-api@sha256:936f194145f04431725cd06db9253da9dde026ea430f87f69837a098a8526284

images[CONTROLLER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-controller@sha256:d156496b7dc81ab4b81c1808390f17cda7400ead44e22af6c7d682fd2d2da07b

images[MUST_GATHER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-must-gather@sha256:3ff09dbc5ca4c0dd196eab5da46a98da7deaa4e71de010cd52684ca2eaaa0d7a

images[OPENSTACK_POPULATOR_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/openstack-populator@sha256:f7bbdd2504f4e8441a9be58ee780b6c6ca3b4285ba4020b8958575dc5ecf9026

images[OPERATOR_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-operator@sha256:e34c608d6439b9ac5f9f297e4eaf35c69a5b8b74e3622385415d33479071f420

images[OVA_PROVIDER_SERVER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/ova-provider-server@sha256:f8358ba912fbb4928f9021f0dd01d290df87facd1f4eed5da912e10aff5378ae

images[OVIRT_POPULATOR_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/ovirt-populator@sha256:53d572527b5b9a3e5e55d19c0f7b9022a08242b80fdf4e2d3b964f75a48117e8

images[POPULATOR_CONTROLLER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/populator-controller@sha256:c127768d14d9c56d487e2356a315f634278a412a9a50ab9c3eda447d5e3d0f2d

images[UI_PLUGIN_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-console-plugin@sha256:1ddb1b13881bb7a486cbd500d249aa359a28b0f54cc72a95ad52fe427755632e

images[VALIDATION_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/validation@sha256:4d04b097253a0e478d5938e2496a3ead842350f74e3972af0f953083bf7fe906

images[VIRT_V2V_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/virt-v2v@sha256:17628ab1f836549509ad80c321f669441d3f15348a5ef151b69fbbcec03ccd30

# Chage the repository of the images to match the repository they would be pushed
# to during a release.

for k in "${!images[@]}"; do
    image="${images[$k]}"
    new_image="${image/redhat-user-workloads\/rh-mtv-1-tenant/kubev2v}"
    export "$k=$new_image"
done

export VERSION="v2.7.0"
