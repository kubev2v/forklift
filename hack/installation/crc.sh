#! /bin/bash

CRC_BIN_DIR="${CRC_BIN_DIR:-$HOME/.local/bin}"
CRC_PRESET="${CRC_PRESET:-okd}"
CRC_CPUS="${CRC_CPUS:-$(grep -c processor /proc/cpuinfo)}"
CRC_MEM="${CRC_MEM:-16384}"
CRC_DISK="${CRC_DISK:-100}"
CRC_BUNDLE="${CRC_BUNDLE}"
CRC_DOWNLOAD_URL="https://developers.redhat.com/content-gateway/rest/mirror/pub/openshift-v4/clients/crc/latest/crc-linux-amd64.tar.xz"
CRC_USE_INTEGRATED_REGISTRY="${CRC_USE_INTEGRATED_REGISTRY}"

KUBEVIRT_VERSION="${KUBEVIRT_VERSION:-$(curl -s https://api.github.com/repos/kubevirt/kubevirt/releases | grep tag_name | grep -v -- '-rc' | sort -r | head -1 | awk -F': ' '{print $2}' | sed 's/,//' | xargs)}"
KUBEVIRT_URL="https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"

ROOTLESS="${ROOTLESS}"

CONTAINER_RUNTIME="${CONTAINER_RUNTIME}"
if [ -z "${CONTAINER_RUNTIME}" ]; then
  CONTAINER_CMD="${CONTAINER_CMD:-$(type -P podman || type -P docker || :)}"
  if [ -z "${CONTAINER_CMD}" ]; then
    echo "Container runtime not detected"
    exit 1
  fi
  CONTAINER_RUNTIME="$(basename ${CONTAINER_CMD})"
else
  CONTAINER_CMD=$(type -P $CONTAINER_RUNTIME)
fi

# Install CRC if not in PATH
if [ ! "$(type -P crc)" ]; then
  [ -d "${CRC_BIN_DIR}" ] || mkdir -p "${CRC_BIN_DIR}"
  curl -sL $CRC_DOWNLOAD_URL | tar -C "${CRC_BIN_DIR}" --strip-components=1 -xJf - */crc
  if ! [[ "$PATH" =~ "${CRC_BIN_DIR}" ]]; then
      export PATH="${CRC_BIN_DIR}:${PATH}"
  fi
fi

# Detect CRC
[ "$(type -P crc)" ] || ( echo "crc is not in PATH" ;  exit 2 )

[ -z "${CRC_PULL_SECRET_FILE}" ] || CRC_OPTS="--pull-secret-file=${CRC_PULL_SECRET_FILE}"

if [ "${CRC_BUNDLE}" ]; then
  crc config set bundle ${CRC_BUNDLE}
else
  crc config unset bundle
fi
crc config set preset ${CRC_PRESET}
crc config set cpus ${CRC_CPUS}
crc config set memory ${CRC_MEM}
crc config set disk-size ${CRC_DISK}
crc config set consent-telemetry no
crc setup
crc start ${CRC_OPTS}

eval $(crc oc-env)

USERNAME=kubeadmin
PASSWORD=$(crc console --credentials --output json | jq -r .clusterConfig.adminCredentials.password)
oc login -u kubeadmin -p "${PASSWORD}" "https://api.crc.testing:6443"

CRC_REGISTRY=$(oc get route -n openshift-image-registry default-route -o 'jsonpath={.spec.host}')
CRC_REGISTRY_CA_CERT=$(oc get secret router-ca -n openshift-ingress-operator -o go-template --template='{{index .data "tls.crt" | base64decode}}')

if [ "${CONTAINER_RUNTIME}" == "podman" ]; then
  if [ "${ROOTLESS}" ]; then
    CRC_CERTS_DIR="${HOME}/.config/containers/certs.d/${CRC_REGISTRY}"
  else
    CRC_CERTS_DIR="/etc/containers/certs.d/${CRC_REGISTRY}"
  fi
fi
if [ "${CONTAINER_RUNTIME}" == "docker" ]; then
  if [ "${ROOTLESS}" ]; then
    CRC_CERTS_DIR="${HOME}/.config/docker/certs.d/${CRC_REGISTRY}"
  else
    CRC_CERTS_DIR="/etc/docker/certs.d/${CRC_REGISTRY}"
  fi
fi

CRC_REGISTRY_SCRIPT="mkdir -p '${CRC_CERTS_DIR}'; echo '${CRC_REGISTRY_CA_CERT}' | openssl x509 -text -out '${CRC_CERTS_DIR}/ca.crt'"
if [ "${ROOTLESS}" ]; then
  bash -c "$CRC_REGISTRY_SCRIPT"
else
  sudo bash -c "$CRC_REGISTRY_SCRIPT"
fi

if [ "${CRC_USE_INTEGRATED_REGISTRY}" ]; then
	${CONTAINER_CMD} login -u "$(oc whoami)" -p "$(oc whoami -t)" "${CRC_REGISTRY}"
else
  export REGISTRY_IP="$(crc ip | cut -f -3 -d .).1"
	export REGISTRY_PORT="${REGISTRY_PORT:-5001}"
	export REGISTRY="${REGISTRY_IP}:${REGISTRY_PORT}"
	$(dirname -- ${BASH_SOURCE[0]})/registry.sh
  oc patch image.config.openshift.io/cluster --type=merge \
			-p "{\"spec\":{\"allowedRegistriesForImport\":[{\"domainName\":\"${REGISTRY}\",\"insecure\":true}],\"registrySources\":{\"insecureRegistries\":[\"${REGISTRY}\"]}}}" --type="merge"
fi

oc apply -f "${KUBEVIRT_URL}"
