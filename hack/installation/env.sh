#! /bin/bash

###################################################################
# Common Variables:
###################################################################
### Container runtime
# export CONTAINER_RUNTIME="podman"
# export ROOTLESS=true
#
### Registry
# export REGISTRY=quay.io
# export REGISTRY_ORG="kubev2v"
# export REGISTRY_TAG="devel"
#
### Operator configuration
# export NAMESPACE="konveyor-forklift"
# export OPERATOR_NAME="forklift-operator"
# export CHANNELS="development"
# export DEFAULT_CHANNEL="development"
#
### Catalog configuration
# export CATALOG_NAMESPACE="konveyor-forklift"
# export CATALOG_NAME="forklift-catalog"
# export CATALOG_DISPLAY_NAME="Konveyor Forklift"
# export CATALOG_PUBLISHER="Community"
#
### Operator Index configuration
# Use OPM_OPTS="--use-http" when using a non HTTPS registry
# Use OPM_OPTS="--skip-tls-verify" when using an HTTPS registry with self-signed certificate
#export OPM_OPTS=""
#
### Default Images
# export CONTROLLER_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-controller:${REGISTRY_TAG}"
# export API_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-api:${REGISTRY_TAG}"
# export VALIDATION_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-validation:${REGISTRY_TAG}"
# export VIRT_V2V_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-virt-v2v:${REGISTRY_TAG}"
# export VIRT_V2V_WARM_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-virt-v2v-warm:${REGISTRY_TAG}"
# export OPERATOR_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-operator:${REGISTRY_TAG}"
# export OPERATOR_BUNDLE_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-operator-bundle:${REGISTRY_TAG}"
# export OPERATOR_INDEX_IMAGE="${REGISTRY}/${REGISTRY_ORG}/forklift-operator-index:${REGISTRY_TAG}"
# export POPULATOR_CONTROLLER_IMAGE="${REGISTRY}/${REGISTRY_ORG}/populator-controller:${REGISTRY_TAG}"
# export OVIRT_POPULATOR_IMAGE="${REGISTRY}/${REGISTRY_ORG}/ovirt-populator:${REGISTRY_TAG}"
# export OPENSTACK_POPULATOR_IMAGE="${REGISTRY}/${REGISTRY_ORG}/openstack-populator:${REGISTRY_TAG}"
#
### External images
# export MUST_GATHER_IMAGE="quay.io/kubev2v/forklift-must-gather:latest"
# export MUST_GATHER_API_IMAGE="quay.io/kubev2v/forklift-must-gather-api:latest"
# export UI_PLUGIN_IMAGE="quay.io/kubev2v/forklift-console-plugin:latest"
###################################################################


###################################################################
# CRC installation options:
###################################################################
# The directory where the 'crc' binary will be installed (this path
# will be added to the PATH variable). (default: ${HOME}/.local/bin)
#CRC_BIN_DIR="$HOME/.local/bin"
#
# Number of CPUS for CRC. By default all of the available CPUs will
# be used
#CRC_CPUS="$(grep -c processor /proc/cpuinfo)}"
#
# Memory for CRC in MB. (default: 16384)
#CRC_MEM="16384"
#
# Disk size used by the CRC installation (default: 100)
# CRC_DISK="100"
#
# Select Openshift/OKD/Podman installation type (default: okd)
#CRC_PRESET="okd"
#
# Pull secret file. If not provided it will be requested at
# installation time by the script
#CRC_PULL_SECRET_FILE=
#
# Bundle to deploy. If not specified the default bundle will be
# installed. OKD default bundle doesn't work for now because of
# expired certificates so the installation script will temporarily
# overwrite it with:
# docker://quay.io/crcont/okd-bundle:4.13.0-0.okd-2023-06-04-080300
#CRC_BUNDLE="${CRC_BUNDLE}"
#
# Use the integrated CRC registry instead of local one. (default: '')
# Non empty variable is considered as true.
# CRC_USE_INTEGRATED_REGISTRY=
###################################################################
# CRC env
###################################################################
# Authenticate with the CRC Openshift API:
# eval $(crc oc-env)
# USERNAME="kubeadmin"
# PASSWORD="$(crc console --credentials --output json | jq -r .clusterConfig.adminCredentials.password)"
# oc login -u "${USERNAME}" -p "${PASSWORD}"  https://api.crc.testing:6443
#
# CRC variables using integrated registry:
# export REGISTRY="default-route-openshift-image-registry.apps-crc.testing"
# export OPM_OPTS="--skip-tls-verify"
# ${CONTAINER_CMD} login -u "$(oc whoami)" -p "$(oc whoami -t)" "${REGISTRY}"
#
# CRC variables using local registry
# export REGISTRY="$(ip route get 1.1.1.1 | grep -oP 'src \K\S+'):5001"
# export OPM_OPTS="--use-http"
# oc patch image.config.openshift.io/cluster -p "{\"spec\":{\"allowedRegistriesForImport\":[{\"domainName\":\"$REGISTRY\",\"insecure\":true}],\"registrySources\":{\"insecureRegistries\":[\"$REGISTRY\"]}}}" --type="merge"
###################################################################


###################################################################
# Openshift/OKD env
###################################################################
# Openshift and registry logins
# USERNAME="kubeadmin"
# PASSWORD=""
# oc login -u "${USERNAME}" -p "${PASSWORD}"  https://api.ocp4.example.com:6443
# ${CONTAINER_CMD} login -u "$(oc whoami)" -p "$(oc whoami -t)" "${REGISTRY}"
#
# Export required variables:
# export REGISTRY="$(oc get route -n openshift-image-registry default-route -o 'jsonpath={.spec.host}')"
# export OPM_OPTS="--skip-tls-verify"
###################################################################


###################################################################
# Minikube installation options
###################################################################
# Driver: kvm2, docker or podman. (default: podman)
#MINIKUBE_DRIVER="${CONTAINER_RUNTIME:-kvm2}"
#
# Minikube number of CPUs (default: max)
#MINIKUBE_CPUS="max"
#
# Minikube memory in MB (default: 16384)
#MINIKUBE_MEMORY="16384"
#
# Minikube addons that will be enabled (default: olm,kubevirt).
#MINIKUBE_ADDONS="olm,kubevirt"
#
# Rootless configuration for docker or podman drivers
# - docker default: false (rootless does not work for now)
# - podman default: true
#ROOTLESS="true"
#
###################################################################
# Minikube env
###################################################################
# Use the local registry created by the installation scripts
# export REGISTRY="$(ip route get 1.1.1.1 | grep -oP 'src \K\S+'):5001"
#
# Use http when building the operator index
# export OPM_OPTS=--use-http
###################################################################


####################################################################
# Kind installation options
###################################################################
# Use docker rootless installation. (default: ''). Non empty variable
# is considered as true.
#ROOTLESS=
#
# Kind version to install (default: v0.15.0)
#KIND_VERSION="v0.15.0"
#
# Operator Livecycle Manager version (default: v.0.25.0)
#OLM_VERSION="v0.25.0"
#
# Cert manager operator version (default: v.1.12.2)
#CERT_MANAGER_VERSION="v1.12.2"
###################################################################
# Kind env
###################################################################
# Use the local registry created by the installation scripts
#export REGISTRY="localhost:5001"
#
# Use http when building the operator index
# export OPM_OPTS=--use-http
#
# Switch kubectl context:
# kind export kubeconfig --name forklift
###################################################################
