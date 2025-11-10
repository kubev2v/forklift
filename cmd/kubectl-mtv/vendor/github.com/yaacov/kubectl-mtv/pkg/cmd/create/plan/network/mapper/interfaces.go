package mapper

import (
	forkliftv1beta1 "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1/ref"
)

// NetworkMappingOptions contains options for network mapping
type NetworkMappingOptions struct {
	DefaultTargetNetwork string
	Namespace            string
	SourceProviderType   string
	TargetProviderType   string
}

// NetworkMapper defines the interface for network mapping operations
type NetworkMapper interface {
	CreateNetworkPairs(sourceNetworks []ref.Ref, targetNetworks []forkliftv1beta1.DestinationNetwork, opts NetworkMappingOptions) ([]forkliftv1beta1.NetworkPair, error)
}
