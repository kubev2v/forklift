#! /bin/bash

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

REGISTRY_IP="${REGISTRY_IP:-localhost}"
REGISTRY_PORT="${REGISTRY_PORT:-5001}"

LOCAL_REGISTRY_NAME="${LOCAL_REGISTRY_NAME:-forklift-registry}"
LOCAL_REGISTRY_IP="${LOCAL_REGISTRY_IP:-localhost}"
LOCAL_REGISTRY_PORT="${LOCAL_REGISTRY_PORT:-5000}"

if [ "$(${CONTAINER_CMD} ps -a -f name=${LOCAL_REGISTRY_NAME} --format={{.Names}})" ]; then
  if [ "$(${CONTAINER_CMD} inspect -f {{.State.Running}} ${LOCAL_REGISTRY_NAME})" != 'true' ]; then
    $CONTAINER_CMD start $LOCAL_REGISTRY_NAME
  fi
else
  ${CONTAINER_CMD} run \
    -d --restart=always -p "0.0.0.0:${REGISTRY_PORT}:${LOCAL_REGISTRY_PORT}" -e REGISTRY_STORAGE_DELETE_ENABLED=true --name "${LOCAL_REGISTRY_NAME}" \
    --network bridge \
    registry:2
fi

REGISTRY="${REGISTRY_IP}:${REGISTRY_PORT}"
LOCAL_REGISTRY="${LOCAL_REGISTRY_IP}:${LOCAL_REGISTRY_PORT}"


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
  REGISTRY_CONF_FILE="${REGISTRY_CONF_DIR}/local-registry.conf"
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

if systemctl is-active -q firewalld && ! firewall-cmd -q --query-port=${REGISTRY_PORT}/tcp --zone libvirt ; then
  echo "Firewalld is active adding registry port to libvirt zone"
  sudo bash -c "firewall-cmd --add-port=${REGISTRY_PORT}/tcp --zone=libvirt; systemctl reload firewalld"
fi
