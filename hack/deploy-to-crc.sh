#!/usr/bin/env bash
#
# This is an opinionated script for deploying the images of all components within
# the repository to an CRC instance. It is assumed that 'oc' is already configured
# to connect to the CRC instance, 'podman' is installed and that the script is
# executed from the root folder of this repository.
# After executing this script there should be an operator named Forklift Operator
# with 'Forklift (devel)' label available in the operator hub.

set -e

CONTAINER_CMD=$(command -v podman)
[ ! -z "${CONTAINER_CMD}" ] || CONTAINER_CMD=$(command -v docker)

if [ -z "${CONTAINER_CMD}" ]; then
  echo "Container runtime not detected"
  exit 1
fi

[[ -z "${REGISTRY}" ]] && export REGISTRY=$(oc get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
[[ -z "${REGISTRY_ORG}" ]] && export REGISTRY_ORG=openshift
[[ -z "${REGISTRY_TAG}" ]] && export REGISTRY_TAG=devel

CERT_PATH=/etc/pki/ca-trust/source/anchors/${REGISTRY}.crt
if [[ ! -f ${CERT_PATH} ]]; then
	oc get secret -n openshift-ingress router-certs-default -o go-template='{{index .data "tls.crt"}}' | base64 -d | sudo tee ${CERT_PATH}  > /dev/null
	sudo update-ca-trust enable
fi

${CONTAINER_CMD} login -u kubeadmin -p $(oc whoami -t) $REGISTRY

make push-all-images

cat << EOF | oc apply -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: forklift-devel
  namespace: openshift-marketplace
spec:
  displayName: Forklift (devel)
  publisher: Konveyor
  sourceType: grpc
  image: image-registry.openshift-image-registry.svc:5000/${REGISTRY_ORG}/forklift-operator-index:${REGISTRY_TAG}
EOF
