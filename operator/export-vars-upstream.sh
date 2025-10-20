#!/bin/bash -ex

declare -A images

# The image refrences below will get updated by Konflux everytime there is a new image.

images[API_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-api@sha256:35f0675e518528d911fcc3d36b343ededd9782fe0a5b7284a229dd9a0afc9656

images[CONTROLLER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-controller@sha256:3a70be74984ec0530496c7a1d66fecda41216a2afa36803eaaa422a832677a23

images[MUST_GATHER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-must-gather@sha256:3ff09dbc5ca4c0dd196eab5da46a98da7deaa4e71de010cd52684ca2eaaa0d7a

images[OPENSTACK_POPULATOR_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/openstack-populator@sha256:f7bbdd2504f4e8441a9be58ee780b6c6ca3b4285ba4020b8958575dc5ecf9026

images[OPERATOR_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-operator@sha256:3e54959b60651873c7410a10dfaa8537d064d304cf34f47680bdaccfc6f3aeba

images[OVA_PROVIDER_SERVER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/ova-provider-server@sha256:fea2945986e40914f615438497b478dee5d3adbcb0e4729c177dee21772e4342

images[OVIRT_POPULATOR_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/ovirt-populator@sha256:c2e933d4ec5c94e7721d2835d71a91d0bd7451af2cffd2860c4dd86e40a0bdfc

images[POPULATOR_CONTROLLER_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/populator-controller@sha256:c127768d14d9c56d487e2356a315f634278a412a9a50ab9c3eda447d5e3d0f2d

images[UI_PLUGIN_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-console-plugin@sha256:86d33a87e4e96858af57c9a574dfc7357b5e97aa78e269de39aa3d7b7c3423d9

images[CLI_DOWNLOAD_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/forklift-cli-download@sha256:0000000000000000000000000000000000000000000000000000000000000000

images[VALIDATION_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/validation@sha256:58edf1e9d49c33bdd62abcd632a61ba61e68b8977f3f57e9feb93c1dd4f07fb6

images[VIRT_V2V_IMAGE]=quay.io/redhat-user-workloads/rh-mtv-1-tenant/forklift-operator/virt-v2v@sha256:9a8548cf4439121ceaa7a8d626d6c6f1f9280f63b2daf471d24fd62c8185a3f4

# Chage the repository of the images to match the repository they would be pushed
# to during a release.

for k in "${!images[@]}"; do
    image="${images[$k]}"
    new_image="${image/redhat-user-workloads\/rh-mtv-1-tenant\/forklift-operator/kubev2v}"
    export "$k=$new_image"
done
