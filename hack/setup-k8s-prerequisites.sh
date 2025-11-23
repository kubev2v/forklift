#!/bin/bash
set -e

# Constants for extracting version from GitHub API response
TAG_NAME_FIELD='tag_name'
EXTRACT_TAG_PATTERN='s/.*"([^"]+)".*/\1/'

echo "Checking cluster connectivity..."
kubectl cluster-info > /dev/null 2>&1 || { echo "Error: No Kubernetes cluster found" >&2; exit 1; }

echo "Checking latest versions..."
CERT_MANAGER_VERSION=$(curl -s https://api.github.com/repos/cert-manager/cert-manager/releases/latest | grep "$TAG_NAME_FIELD" | sed -E "$EXTRACT_TAG_PATTERN")
CDI_VERSION=$(curl -s https://api.github.com/repos/kubevirt/containerized-data-importer/releases/latest | grep "$TAG_NAME_FIELD" | sed -E "$EXTRACT_TAG_PATTERN")
CNA_VERSION=$(curl -s https://api.github.com/repos/kubevirt/cluster-network-addons-operator/releases/latest | grep "$TAG_NAME_FIELD" | sed -E "$EXTRACT_TAG_PATTERN")
VIRT_VERSION=$(curl -s https://api.github.com/repos/kubevirt/kubevirt/releases/latest | grep "$TAG_NAME_FIELD" | sed -E "$EXTRACT_TAG_PATTERN")

# Install cert-manager
echo "Installing cert-manager ${CERT_MANAGER_VERSION}..."
kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
kubectl wait --for=condition=Available --timeout=300s deployment -n cert-manager --all

# Install CDI
echo "Installing CDI ${CDI_VERSION}..."
kubectl create -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml" --dry-run=client -o yaml | kubectl apply -f -
kubectl create -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml" --dry-run=client -o yaml | kubectl apply -f -
kubectl wait --for=condition=Available --timeout=300s deployment -n cdi cdi-operator

# Install CNA
echo "Installing Cluster Network Addons ${CNA_VERSION}..."
kubectl apply -f "https://github.com/kubevirt/cluster-network-addons-operator/releases/download/${CNA_VERSION}/namespace.yaml"
kubectl apply -f "https://github.com/kubevirt/cluster-network-addons-operator/releases/download/${CNA_VERSION}/network-addons-config.crd.yaml"
kubectl apply -f "https://github.com/kubevirt/cluster-network-addons-operator/releases/download/${CNA_VERSION}/operator.yaml"
kubectl wait --for=condition=Available --timeout=300s deployment -n cluster-network-addons cluster-network-addons-operator

# Configure NetworkAddonsConfig
echo "Configuring network addons..."
cat << EOF | kubectl apply -f -
apiVersion: networkaddonsoperator.network.kubevirt.io/v1
kind: NetworkAddonsConfig
metadata:
  name: cluster
  namespace: cluster-network-addons
spec:
  multus: {}
  linuxBridge: {}
  macvtap: {}
  imagePullPolicy: Always
EOF
kubectl wait --for=condition=Available --timeout=300s networkaddonsconfig cluster

# Install KubeVirt
echo "Installing KubeVirt ${VIRT_VERSION}..."
kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${VIRT_VERSION}/kubevirt-operator.yaml" --dry-run=client -o yaml | kubectl apply -f -
kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${VIRT_VERSION}/kubevirt-cr.yaml" --dry-run=client -o yaml | kubectl apply -f -
kubectl wait --for=condition=Available --timeout=300s deployment -n kubevirt virt-operator

# Create example NetworkAttachmentDefinition
echo "Creating NetworkAttachmentDefinition..."
until kubectl get crd network-attachment-definitions.k8s.cni.cncf.io > /dev/null 2>&1; do
  sleep 5
done
cat << EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: example
  namespace: cluster-network-addons
spec:
  config: '{}'
EOF

# Install OLM
echo "Installing OLM..."
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/crds.yaml
kubectl apply -f https://raw.githubusercontent.com/operator-framework/operator-lifecycle-manager/master/deploy/upstream/quickstart/olm.yaml
kubectl wait --for=condition=Available --timeout=300s deployment -n olm olm-operator

echo ""
echo "Installed versions:"
echo "  cert-manager: ${CERT_MANAGER_VERSION}"
echo "  CDI: ${CDI_VERSION}"
echo "  Cluster Network Addons: ${CNA_VERSION}"
echo "  KubeVirt: ${VIRT_VERSION}"
echo "  OLM: latest"
echo ""
echo "We are ready to deploy forklift"

