package ovirt

import (
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper"
)

// OvirtNetworkMapper implements network mapping for oVirt providers
type OvirtNetworkMapper struct{}

// NewOvirtNetworkMapper creates a new oVirt network mapper
func NewOvirtNetworkMapper() mapper.NetworkMapper {
	return &OvirtNetworkMapper{}
}

// CreateNetworkPairs creates network mapping pairs using generic logic (no same-name matching)
func (m *OvirtNetworkMapper) CreateNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) ([]forkliftv1beta1.NetworkPair, error) {
	var networkPairs []forkliftv1beta1.NetworkPair

	klog.V(4).Infof("DEBUG: oVirt network mapper - Creating network pairs - %d source networks, %d target networks", len(sourceNetworks), len(targetNetworks))

	if len(sourceNetworks) == 0 {
		klog.V(4).Infof("DEBUG: No source networks to map")
		return networkPairs, nil
	}

	// Use generic default behavior (first -> default, others -> ignored)
	defaultDestination := findDefaultTargetNetwork(targetNetworks, opts)
	klog.V(4).Infof("DEBUG: Selected default target network: %s/%s (%s)",
		defaultDestination.Namespace, defaultDestination.Name, defaultDestination.Type)

	// Map the first source network to the default target network
	// Set all other source networks to target "ignored"
	for i, sourceNetwork := range sourceNetworks {
		var destination forkliftv1beta1.DestinationNetwork

		if i == 0 {
			// Map first source network to default target network
			destination = defaultDestination
			klog.V(4).Infof("DEBUG: Mapping first source network %s to default target %s/%s (%s)",
				sourceNetwork.Name, destination.Namespace, destination.Name, destination.Type)
		} else {
			// Set all other source networks to "ignored"
			destination = forkliftv1beta1.DestinationNetwork{Type: "ignored"}
			klog.V(4).Infof("DEBUG: Setting source network %s to ignored", sourceNetwork.Name)
		}

		networkPairs = append(networkPairs, forkliftv1beta1.NetworkPair{
			Source:      sourceNetwork,
			Destination: destination,
		})
	}

	return networkPairs, nil
}

// findDefaultTargetNetwork finds the default target network using the original priority logic
func findDefaultTargetNetwork(targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) forkliftv1beta1.DestinationNetwork {
	// Priority 1: If user explicitly specified a default target network, use it
	if opts.DefaultTargetNetwork != "" {
		defaultDestination := parseDefaultNetwork(opts.DefaultTargetNetwork, opts.Namespace)
		klog.V(4).Infof("DEBUG: Using user-defined default target network: %s/%s (%s)",
			defaultDestination.Namespace, defaultDestination.Name, defaultDestination.Type)
		return defaultDestination
	}

	// Priority 2: Find the first available multus network
	for _, targetNetwork := range targetNetworks {
		if targetNetwork.Type == "multus" {
			klog.V(4).Infof("DEBUG: Using first available multus network as default: %s/%s",
				targetNetwork.Namespace, targetNetwork.Name)
			return targetNetwork
		}
	}

	// Priority 3: Fall back to pod networking if no multus networks available
	klog.V(4).Infof("DEBUG: No user-defined or multus networks available, falling back to pod networking")
	return forkliftv1beta1.DestinationNetwork{Type: "pod"}
}

// parseDefaultNetwork parses a default network specification (from original mapper)
func parseDefaultNetwork(defaultTargetNetwork, namespace string) forkliftv1beta1.DestinationNetwork {
	if defaultTargetNetwork == "default" {
		return forkliftv1beta1.DestinationNetwork{Type: "pod"}
	}

	if defaultTargetNetwork == "ignored" {
		return forkliftv1beta1.DestinationNetwork{Type: "ignored"}
	}

	// Handle "namespace/name" format for multus networks
	if parts := strings.Split(defaultTargetNetwork, "/"); len(parts) == 2 {
		destNetwork := forkliftv1beta1.DestinationNetwork{
			Type: "multus",
			Name: parts[1],
		}
		// Always set namespace, use plan namespace if empty
		if parts[0] != "" {
			destNetwork.Namespace = parts[0]
		} else {
			destNetwork.Namespace = namespace
		}
		return destNetwork
	}

	// Just a name, use the plan namespace
	destNetwork := forkliftv1beta1.DestinationNetwork{
		Type:      "multus",
		Name:      defaultTargetNetwork,
		Namespace: namespace,
	}
	return destNetwork
}
