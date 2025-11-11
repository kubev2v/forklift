package openshift

import (
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper"
)

// OpenShiftNetworkMapper implements network mapping for OpenShift providers
type OpenShiftNetworkMapper struct{}

// NewOpenShiftNetworkMapper creates a new OpenShift network mapper
func NewOpenShiftNetworkMapper() mapper.NetworkMapper {
	return &OpenShiftNetworkMapper{}
}

// CreateNetworkPairs creates network mapping pairs with OpenShift-specific logic
func (m *OpenShiftNetworkMapper) CreateNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) ([]forkliftv1beta1.NetworkPair, error) {
	var networkPairs []forkliftv1beta1.NetworkPair

	klog.V(4).Infof("DEBUG: OpenShift network mapper - Creating network pairs - %d source networks, %d target networks", len(sourceNetworks), len(targetNetworks))
	klog.V(4).Infof("DEBUG: Source provider type: %s, Target provider type: %s", opts.SourceProviderType, opts.TargetProviderType)

	if len(sourceNetworks) == 0 {
		klog.V(4).Infof("DEBUG: No source networks to map")
		return networkPairs, nil
	}

	// For OCP-to-OCP: Try same-name matching (all-or-nothing, respecting uniqueness constraints)
	if opts.TargetProviderType == "openshift" {
		klog.V(4).Infof("DEBUG: OCP-to-OCP migration detected, attempting same-name matching")
		if canMatchAllNetworksByName(sourceNetworks, targetNetworks) {
			klog.V(4).Infof("DEBUG: All networks can be matched by name, using same-name mapping")
			return createSameNameNetworkPairs(sourceNetworks, targetNetworks)
		}
		klog.V(4).Infof("DEBUG: Not all networks can be matched by name, falling back to default behavior")
	}

	// Fall back to default behavior
	return createDefaultNetworkPairs(sourceNetworks, targetNetworks, opts)
}

// canMatchAllNetworksByName checks if every source network can be uniquely matched to a target network by name
func canMatchAllNetworksByName(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork) bool {
	// Create a map of target network names for quick lookup (only multus networks can be matched by name)
	targetNames := make(map[string]bool)
	for _, target := range targetNetworks {
		if target.Type == "multus" && target.Name != "" {
			targetNames[target.Name] = true
		}
	}

	klog.V(4).Infof("DEBUG: Available target networks for name matching: %v", getTargetNetworkNames(targetNetworks))

	// Check if every source has a matching target by name
	// Also ensure we don't have more sources than available targets (uniqueness constraint)
	if len(sourceNetworks) > len(targetNames) {
		klog.V(4).Infof("DEBUG: More source networks (%d) than available target networks (%d) for name matching", len(sourceNetworks), len(targetNames))
		return false
	}

	for _, source := range sourceNetworks {
		if !targetNames[source.Name] {
			klog.V(4).Infof("DEBUG: Source network '%s' has no matching target by name", source.Name)
			return false
		}
	}

	klog.V(4).Infof("DEBUG: All source networks can be matched by name")
	return true
}

// createSameNameNetworkPairs creates network pairs using same-name matching
func createSameNameNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork) ([]forkliftv1beta1.NetworkPair, error) {
	var networkPairs []forkliftv1beta1.NetworkPair

	// Create a map of target networks by name for quick lookup (only multus networks)
	targetByName := make(map[string]forkliftv1beta1.DestinationNetwork)
	for _, target := range targetNetworks {
		if target.Type == "multus" && target.Name != "" {
			targetByName[target.Name] = target
		}
	}

	// Create pairs using same-name matching
	for _, sourceNetwork := range sourceNetworks {
		if targetNetwork, exists := targetByName[sourceNetwork.Name]; exists {
			networkPairs = append(networkPairs, forkliftv1beta1.NetworkPair{
				Source:      sourceNetwork,
				Destination: targetNetwork,
			})
			klog.V(4).Infof("DEBUG: Mapped source network %s -> %s/%s (same name)",
				sourceNetwork.Name, targetNetwork.Namespace, targetNetwork.Name)
		}
	}

	klog.V(4).Infof("DEBUG: Created %d same-name network pairs", len(networkPairs))
	return networkPairs, nil
}

// createDefaultNetworkPairs creates network pairs using the default behavior (first -> default, others -> ignored)
func createDefaultNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) ([]forkliftv1beta1.NetworkPair, error) {
	var networkPairs []forkliftv1beta1.NetworkPair

	// Find the default target network using the original logic
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

	klog.V(4).Infof("DEBUG: Created %d default network pairs", len(networkPairs))
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

// getTargetNetworkNames returns a slice of target network names for logging
func getTargetNetworkNames(targetNetworks []forkliftv1beta1.DestinationNetwork) []string {
	var names []string
	for _, target := range targetNetworks {
		if target.Type == "multus" && target.Name != "" {
			names = append(names, target.Name)
		}
	}
	return names
}
