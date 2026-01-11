package builder

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/mapping"
)

// findStorageMapping looks up target storage class for EBS volume type (gp2, gp3, io1, etc).
// Returns storage class name from StorageMap or empty string if no mapping found.
func (r *Builder) findStorageMapping(volumeType string) string {
	return mapping.FindStorageClass(r.Map.Storage, volumeType)
}

// findNetworkMapping finds the network mapping for a given subnet ID.
// Returns the matching NetworkPair or nil if no mapping found.
func (r *Builder) findNetworkMapping(subnetID string, netMap []api.NetworkPair) *api.NetworkPair {
	// For backward compatibility, we accept netMap as a parameter
	// but the shared function works on the full NetworkMap
	return mapping.FindNetworkPair(r.Map.Network, subnetID)
}
