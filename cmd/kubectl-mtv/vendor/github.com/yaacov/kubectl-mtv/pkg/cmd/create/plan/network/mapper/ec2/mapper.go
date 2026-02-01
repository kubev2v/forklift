package ec2

import (
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
	"k8s.io/klog/v2"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper"
)

// EC2NetworkMapper implements network mapping for EC2 providers
type EC2NetworkMapper struct{}

// NewEC2NetworkMapper creates a new EC2 network mapper
func NewEC2NetworkMapper() mapper.NetworkMapper {
	return &EC2NetworkMapper{}
}

// CreateNetworkPairs creates network mapping pairs for EC2 -> OpenShift migrations
// Mapping strategy:
// - First network (typically public subnet or VPC) -> pod networking (default)
// - All other networks -> ignored
func (m *EC2NetworkMapper) CreateNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) ([]forkliftv1beta1.NetworkPair, error) {
	var networkPairs []forkliftv1beta1.NetworkPair

	klog.V(4).Infof("DEBUG: EC2 network mapper - Creating network pairs - %d source networks", len(sourceNetworks))

	if len(sourceNetworks) == 0 {
		klog.V(4).Infof("DEBUG: No source networks to map")
		return networkPairs, nil
	}

	// Determine the default destination network
	var defaultDestination forkliftv1beta1.DestinationNetwork

	if opts.DefaultTargetNetwork != "" {
		// User specified a default target network (trim whitespace for better UX)
		defaultDestination = parseDefaultNetwork(strings.TrimSpace(opts.DefaultTargetNetwork), opts.Namespace)
		klog.V(4).Infof("DEBUG: Using user-defined default target network: %s/%s (%s)",
			defaultDestination.Namespace, defaultDestination.Name, defaultDestination.Type)
	} else {
		// Default to pod networking for EC2 migrations
		defaultDestination = forkliftv1beta1.DestinationNetwork{Type: "pod"}
		klog.V(4).Infof("DEBUG: Using default pod networking for EC2 migration")
	}

	// Map the first source network to the default target
	// Map all other source networks to "ignored"
	for i, sourceNetwork := range sourceNetworks {
		var destination forkliftv1beta1.DestinationNetwork

		if i == 0 {
			// Map first source network to default target network
			destination = defaultDestination
			klog.V(4).Infof("DEBUG: Mapping first EC2 network %s to target %s/%s (%s)",
				sourceNetwork.ID, destination.Namespace, destination.Name, destination.Type)
		} else {
			// Set all other source networks to "ignored"
			destination = forkliftv1beta1.DestinationNetwork{Type: "ignored"}
			klog.V(4).Infof("DEBUG: Setting EC2 network %s to ignored", sourceNetwork.ID)
		}

		networkPairs = append(networkPairs, forkliftv1beta1.NetworkPair{
			Source:      sourceNetwork,
			Destination: destination,
		})
	}

	klog.V(4).Infof("DEBUG: EC2 network mapper - Created %d network pairs", len(networkPairs))
	return networkPairs, nil
}

// parseDefaultNetwork parses the default network string into a DestinationNetwork
func parseDefaultNetwork(defaultNetwork, namespace string) forkliftv1beta1.DestinationNetwork {
	// Handle special cases
	if defaultNetwork == "default" || defaultNetwork == "" {
		return forkliftv1beta1.DestinationNetwork{Type: "pod"}
	}

	// Parse namespace/name format
	var targetNamespace, targetName string
	if ns, name, found := strings.Cut(defaultNetwork, "/"); found {
		if ns == "" {
			// Format: /name (use provided namespace)
			targetNamespace = namespace
		} else {
			targetNamespace = ns
		}
		targetName = name
	} else {
		// Just a name, use provided namespace
		targetNamespace = namespace
		targetName = defaultNetwork
	}

	// If name is empty after parsing, fall back to pod networking
	if targetName == "" {
		return forkliftv1beta1.DestinationNetwork{Type: "pod"}
	}

	return forkliftv1beta1.DestinationNetwork{
		Type:      "multus",
		Namespace: targetNamespace,
		Name:      targetName,
	}
}
