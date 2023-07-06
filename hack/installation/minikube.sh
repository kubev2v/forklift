#! /bin/bash

export MINIKUBE_DRIVER="${MINIKUBE_DRIVER:-podman}"
export MINIKUBE_PROFILE_NAME="forklift-${MINIKUBE_DRIVER}"
export MINIKUBE_CPUS="${MINIKUBE_CPUS:-max}"
export MINIKUBE_MEMORY="${MINIKUBE_MEMORY:-16384}"
export MINIKUBE_ADDONS="${MINIKUBE_ADDONS:-olm,kubevirt}"
export MINIKUBE_ROOTLESS="${ROOTLESS}"
export MINIKUBE_USE_INTEGRATED_REGISTRY="${MINIKUBE_USE_INTEGRATED_REGISTRY}"

export REGISTRY_IP="localhost"
export REGISTRY_PORT="${REGISTRY_PORT:-5001}"

export LOCAL_REGISTRY_NAME="forklift-registry"
export LOCAL_REGISTRY_IP="localhost"
export LOCAL_REGISTRY_PORT="5000"

MINIKUBE_BIN_DIR="${MINIKUBE_BIN_DIR:-$HOME/.local/bin}"
MINIKUBE_DOWNLOAD_URL="https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64"

CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.12.2}"
CERT_MANAGER_URL="https://github.com/jetstack/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"

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

if [ ! "$(type -P minikube)" ]; then
  [ -d "${MINIKUBE_BIN_DIR}" ] || mkdir -p "${MINIKUBE_BIN_DIR}"
  curl -L $MINIKUBE_DOWNLOAD_URL -o "${MINIKUBE_BIN_DIR}/minikube"
  if ! [[ "$PATH" =~ "${MINIKUBE_BIN_DIR}" ]]; then
      export PATH="${MINIKUBE_BIN_DIR}:${PATH}"
  fi
fi

[ "$(type -P minikube)" ] || ( echo "minikube is not in PATH" ;  exit 2 )

minikube config set profile ${MINIKUBE_PROFILE_NAME}
minikube config set driver ${MINIKUBE_DRIVER}
minikube config set cpus ${MINIKUBE_CPUS}
minikube config set memory ${MINIKUBE_MEMORY}

[ -z "${MINIKUBE_ROOTLESS}" -a "${MINIKUBE_DRIVER}" != "podman" ] || MINIKUBE_ROOTLESS="true"

if [ "${MINIKUBE_ROOTLESS}" ]; then
  minikube config set rootless true
  minikube config set container-runtime containerd
  if [ "${MINIKUBE_DRIVER}" == "docker" ]; then
    [ "$(${CONTAINER_CMD} context ls --format json | jq -r '. | select(.Name == "rootless").Name')" == "rootless" ] || dockerd-rootless-setuptool.sh install -f
    ${CONTAINER_CMD} context use rootless
  fi
else
  minikube config unset rootless
  minikube config unset container-runtime
fi

if [ "${MINIKUBE_USE_INTEGRATED_REGISTRY}" ]; then
  if [[ ! "${MIKIKUBE_ADDONS}" =~ "registry" ]] ; then
    [ ! -z "${MIKIKUBE_ADDONS}" ] || MINIKUBE_ADDONS="${MINIKUBE_ADDONS},"
    MINIKUBE_ADDONS="${MINIKUBE_ADDONS}registry"
  fi
fi

[ -z "${MINIKUBE_ADDONS}" ] || ADDONS_ARGS="--addons=${MINIKUBE_ADDONS}"

minikube start ${ADDONS_ARGS} ${INSECURE_REGISTRY_ARGS} ${REGISTRY_MIRROR_ARGS}
kubectl apply -f "${CERT_MANAGER_URL}"

if [ ! "${MINIKUBE_USE_INTEGRATED_REGISTRY}" ]; then

  if [ "${MINIKUBE_DRIVER}" == "kvm2" ]; then
    REGISTRY_IP="$(minikube ip| cut -f -3 -d .).1"
    REGISTRY_PORT="5001"
    REGISTRY="${REGISTRY_IP}:${REGISTRY_PORT}"
    REGISTRY_MIRROR_DIR="/etc/containerd/certs.d/${REGISTRY_IP}:${REGISTRY_PORT}"
    REGISTRY_MIRROR_CONF="server = \\\"http://$REGISTRY_IP:$REGISTRY_PORT\\\"\n\n[host.\\\"http://${REGISTRY_IP}:${REGISTRY_PORT}\\\"]\n  skip_verify=true"
  else
    REGISTRY_IP="localhost"
    REGISTRY="${REGISTRY_IP}:${REGISTRY_PORT}"
    REGISTRY_MIRROR_DIR="/etc/containerd/certs.d/${REGISTRY_IP}:${REGISTRY_PORT}"
    REGISTRY_MIRROR_CONF="server = \\\"http://$REGISTRY_IP:$REGISTRY_PORT\\\"\n\n[host.\\\"http://${LOCAL_REGISTRY_NAME}:${LOCAL_REGISTRY_PORT}\\\"]\n  skip_verify=true"
  fi

  $(dirname -- ${BASH_SOURCE[0]})/registry.sh

  LOCAL_REGISTRY_CONNECTED=$(${CONTAINER_CMD} inspect -f='{{json .NetworkSettings.Networks}}' "${LOCAL_REGISTRY_NAME}" | jq ".[\"${MINIKUBE_PROFILE_NAME}\"]")
  if [ "$MINIKUBE_DRIVER" != "kvm2" -a -z "${MINIKUBE_USE_INTEGRATED_REGISTRY}" -a "${LOCAL_REGISTRY_CONNECTED}" == "null" ]; then
    echo "Connecting '$LOCAL_REGISTRY_NAME' to '$MINIKUBE_PROFILE_NAME' network"
    ${CONTAINER_CMD} network connect "$MINIKUBE_PROFILE_NAME" "${LOCAL_REGISTRY_NAME}"
  fi

else
  if [ "${MINIKUBE_DRIVER}" == "kvm2" ]; then
    REGISTRY_IP="$(minikube ip)"
    REGISTRY_PORT="5000"
    REGISTRY="${REGISTRY_IP}:${REGISTRY_PORT}"
    REGISTRY_MIRROR_DIR="/etc/containerd/certs.d/${REGISTRY_IP}:${REGISTRY_PORT}"
    REGISTRY_MIRROR_CONF="server = \\\"http://$REGISTRY_IP:$REGISTRY_PORT\\\"\n\n[host.\\\"http://${REGISTRY_IP}:${REGISTRY_PORT}\\\"]\n  skip_verify=true"
  else
    REGISTRY="${REGISTRY_IP}:${REGISTRY_PORT}"
    REGISTRY_PORT_PATH='.[].HostConfig.PortBindings["5000/tcp"][].HostPort'
    [ "$CONTAINER_RUNTIME" == "podman " ] || REGISTRY_PORT_PATH='.[].NetworkSettings.Ports."5000/tcp"[0].HostPort'
    REGISTRY="localhost:$($CONTAINER_RUNTIME inspect ${MINIKUBE_PROFILE_NAME} | jq -r ${REGISTRY_PORT_PATH})"
    REGISTRY_MIRROR_DIR="/etc/containerd/certs.d/${REGISTRY}"
    REGISTRY_MIRROR_CONF="server = \\\"http://${REGISTRY}\\\"\n\n[host.\\\"http://${LOCAL_REGISTRY_IP}:${LOCAL_REGISTRY_PORT}\\\"]\n  skip_verify=true"
  fi

  if [ "${CONTAINER_RUNTIME}" == "podman" ]; then
    if [ "${ROOTLESS}" ]; then
      REGISTRY_CONF_DIR="${HOME}/.config/containers/registries.conf.d"
    else
      REGISTRY_CONF_DIR="/etc/containers/registries.conf.d"
    fi
  fi

  if [ "${CONTAINER_RUNTIME}" == "docker" ]; then
    if [ "${ROOTLESS}" ]; then
      REGISTRY_CONF_DIR="${HOME}/.config/docker"
    else
      REGISTRY_CONF_DIR="/etc/docker"
    fi
  fi

  if [ ! -d "${REGISTRY_CONF_DIR}" ]; then
    echo "The '${REGISTRY_CONF_DIR}' does not exist, creating it..."
    if [ "${ROOTLESS}" ]; then
      mkdir -p "${REGISTRY_CONF_DIR}"
    else
      sudo mkdir -p "${REGISTRY_CONF_DIR}"
    fi
  fi

  if [ "${CONTAINER_RUNTIME}" == "podman" ]; then
    REGISTRY_CONF_FILE="${REGISTRY_CONF_DIR}/minikube.conf"
    REGISTRY_INSECURE_CONFIG=$(echo -e "[[registry]]\nlocation = \"${REGISTRY}\"\ninsecure = true\n")
  fi

  if [ "${CONTAINER_RUNTIME}" == "docker" ]; then
    REGISTRY_CONF_FILE="${REGISTRY_CONF_DIR}/daemon.json"
    if [ ! -f "${REGISTRY_CONF_FILE}" ]; then
      echo "'$REGISTRY_CONF_FILE' file does not exist, creating it...";
      if [ "${ROOTLESS}" ]; then
        echo "{}" > "${REGISTRY_CONF_FILE}"
      else
        sudo bash -c "echo {} > ${REGISTRY_CONF_FILE}"
      fi
    fi
    REGISTRY_INSECURE_CONFIG=$(jq -r ". | if .[\"insecure-registries\"] then .[\"insecure-registries\"] |= (. + [\"${REGISTRY}\"] | unique) else . |= {\"insecure-registries\": [\"${REGISTRY}\"]} end" ${REGISTRY_CONF_FILE})
  fi

  echo "Adding the registry '${REGISTRY}' to the insecure registries in '${REGISTRY_CONF_FILE}'"
  REGISTRY_INSECURE_SCRIPT="echo '${REGISTRY_INSECURE_CONFIG}' > ${REGISTRY_CONF_FILE}"
  if [ "${ROOTLESS}" ]; then
    bash -c "${REGISTRY_INSECURE_SCRIPT}"
    [ "${CONTAINER_RUNTIME}" != "docker" ] || systemctl --user reload docker
  else
    sudo bash -c "${REGISTRY_INSECURE_SCRIPT}"
    [ "${CONTAINER_RUNTIME}" != "docker" ] || sudo systemctl reload docker
  fi
fi

minikube ssh "sudo mkdir -p \"${REGISTRY_MIRROR_DIR}\""
minikube ssh "sudo bash -c 'echo -e \"${REGISTRY_MIRROR_CONF}\" > ${REGISTRY_MIRROR_DIR}/hosts.toml'"
minikube ssh "sudo systemctl restart containerd"
