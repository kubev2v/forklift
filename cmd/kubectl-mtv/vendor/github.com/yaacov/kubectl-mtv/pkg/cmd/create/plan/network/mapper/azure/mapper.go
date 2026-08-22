package azure

import (
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper"
)

// AzureNetworkMapper implements network mapping for Azure providers
type AzureNetworkMapper struct{}

// NewAzureNetworkMapper creates a new Azure network mapper
func NewAzureNetworkMapper() mapper.NetworkMapper {
	return &AzureNetworkMapper{}
}

// CreateNetworkPairs creates network mapping pairs for Azure -> OpenShift migrations
// Mapping strategy:
// - First network (subnet) -> pod networking (default) or user-specified NAD
// - All other networks -> ignored
func (m *AzureNetworkMapper) CreateNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) ([]forkliftv1beta1.NetworkPair, error) {
	var networkPairs []forkliftv1beta1.NetworkPair

	klog.V(4).Infof("DEBUG: Azure network mapper - Creating network pairs - %d source networks", len(sourceNetworks))

	if len(sourceNetworks) == 0 {
		klog.V(4).Infof("DEBUG: No source networks to map")
		return networkPairs, nil
	}

	var defaultDestination forkliftv1beta1.DestinationNetwork

	if opts.DefaultTargetNetwork != "" {
		defaultDestination = parseDefaultNetwork(strings.TrimSpace(opts.DefaultTargetNetwork), opts.Namespace)
		klog.V(4).Infof("DEBUG: Using user-defined default target network: %s/%s (%s)",
			defaultDestination.Namespace, defaultDestination.Name, defaultDestination.Type)
	} else {
		defaultDestination = forkliftv1beta1.DestinationNetwork{Type: "pod"}
		klog.V(4).Infof("DEBUG: Using default pod networking for Azure migration")
	}

	for i, sourceNetwork := range sourceNetworks {
		var destination forkliftv1beta1.DestinationNetwork

		if i == 0 {
			destination = defaultDestination
			klog.V(4).Infof("DEBUG: Mapping first Azure network %s to target %s/%s (%s)",
				sourceNetwork.ID, destination.Namespace, destination.Name, destination.Type)
		} else {
			destination = forkliftv1beta1.DestinationNetwork{Type: "ignored"}
			klog.V(4).Infof("DEBUG: Setting Azure network %s to ignored", sourceNetwork.ID)
		}

		networkPairs = append(networkPairs, forkliftv1beta1.NetworkPair{
			Source:      forkliftv1beta1.NetworkSourceRef{Ref: sourceNetwork},
			Destination: destination,
		})
	}

	klog.V(4).Infof("DEBUG: Azure network mapper - Created %d network pairs", len(networkPairs))
	return networkPairs, nil
}

func parseDefaultNetwork(defaultNetwork, namespace string) forkliftv1beta1.DestinationNetwork {
	if defaultNetwork == "default" || defaultNetwork == "" {
		return forkliftv1beta1.DestinationNetwork{Type: "pod"}
	}

	var targetNamespace, targetName string
	if ns, name, found := strings.Cut(defaultNetwork, "/"); found {
		if ns == "" {
			targetNamespace = namespace
		} else {
			targetNamespace = ns
		}
		targetName = name
	} else {
		targetNamespace = namespace
		targetName = defaultNetwork
	}

	if targetName == "" {
		return forkliftv1beta1.DestinationNetwork{Type: "pod"}
	}

	return forkliftv1beta1.DestinationNetwork{
		Type:      "multus",
		Namespace: targetNamespace,
		Name:      targetName,
	}
}
