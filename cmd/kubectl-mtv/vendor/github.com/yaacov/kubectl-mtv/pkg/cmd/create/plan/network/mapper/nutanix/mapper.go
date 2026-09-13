package nutanix

import (
	"strings"

	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"

	"github.com/yaacov/kubectl-mtv/pkg/cmd/create/plan/network/mapper"
)

// NetworkMapper implements network mapping for Nutanix providers.
type NetworkMapper struct{}

// NewNetworkMapper creates a Nutanix network mapper.
func NewNetworkMapper() mapper.NetworkMapper {
	return &NetworkMapper{}
}

// CreateNetworkPairs maps the first source network to the default target and ignores the rest.
func (m *NetworkMapper) CreateNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) ([]forkliftv1beta1.NetworkPair, error) {
	var networkPairs []forkliftv1beta1.NetworkPair
	if len(sourceNetworks) == 0 {
		return networkPairs, nil
	}

	defaultDestination := findDefaultTargetNetwork(targetNetworks, opts)
	for i, sourceNetwork := range sourceNetworks {
		destination := forkliftv1beta1.DestinationNetwork{Type: "ignored"}
		if i == 0 {
			destination = defaultDestination
		}
		networkPairs = append(networkPairs, forkliftv1beta1.NetworkPair{
			Source:      forkliftv1beta1.NetworkSourceRef{Ref: sourceNetwork},
			Destination: destination,
		})
	}
	return networkPairs, nil
}

func findDefaultTargetNetwork(targetNetworks []forkliftv1beta1.DestinationNetwork, opts mapper.NetworkMappingOptions) forkliftv1beta1.DestinationNetwork {
	if opts.DefaultTargetNetwork != "" {
		return parseDefaultNetwork(opts.DefaultTargetNetwork, opts.Namespace)
	}
	for _, targetNetwork := range targetNetworks {
		if targetNetwork.Type == "multus" {
			return targetNetwork
		}
	}
	return forkliftv1beta1.DestinationNetwork{Type: "pod"}
}

func parseDefaultNetwork(defaultTargetNetwork, namespace string) forkliftv1beta1.DestinationNetwork {
	if defaultTargetNetwork == "default" {
		return forkliftv1beta1.DestinationNetwork{Type: "pod"}
	}
	if defaultTargetNetwork == "ignored" {
		return forkliftv1beta1.DestinationNetwork{Type: "ignored"}
	}
	if parts := strings.Split(defaultTargetNetwork, "/"); len(parts) == 2 && parts[1] != "" {
		destNetwork := forkliftv1beta1.DestinationNetwork{Type: "multus", Name: parts[1]}
		if parts[0] != "" {
			destNetwork.Namespace = parts[0]
		} else {
			destNetwork.Namespace = namespace
		}
		return destNetwork
	}
	return forkliftv1beta1.DestinationNetwork{Type: "multus", Name: defaultTargetNetwork, Namespace: namespace}
}
