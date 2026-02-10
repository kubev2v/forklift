package base

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/mapping"
)

// FindStorageMapping looks up target storage class for EBS volume type (gp2, gp3, io1, etc).
func (r *Base) FindStorageMapping(volumeType string) string {
	return mapping.FindStorageClass(r.Map.Storage, volumeType)
}

// FindNetworkMapping finds the network mapping for a given subnet ID.
func (r *Base) FindNetworkMapping(subnetID string, netMap []api.NetworkPair) *api.NetworkPair {
	return mapping.FindNetworkPair(r.Map.Network, subnetID)
}

// FindMappingForSubnet is a convenience wrapper that accepts a *string subnet ID.
// Returns nil when subnetID is nil or no mapping is found.
func (r *Base) FindMappingForSubnet(subnetID *string) *api.NetworkPair {
	if subnetID == nil {
		return nil
	}
	return mapping.FindNetworkPair(r.Map.Network, *subnetID)
}
