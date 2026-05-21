package builder

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/provider/ec2/controller/mapping"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// findStorageMapping looks up target storage class for EBS volume type (gp2, gp3, io1, etc).
// Returns storage class name from StorageMap or empty string if no mapping found.
func (r *Builder) findStorageMapping(volumeType string) string {
	return mapping.FindStorageClass(r.Map.Storage, volumeType)
}

func (r *Builder) buildNICResolver(enis []model.InstanceNetworkInterface) ([]string, map[string][]api.NetworkPair) {
	pairsBySource := map[string][]api.NetworkPair{}
	if r.Map.Network != nil {
		for _, pair := range r.Map.Network.Spec.Map {
			if pair.Source.ID != "" {
				pairsBySource[pair.Source.ID] = append(pairsBySource[pair.Source.ID], pair)
			}
			if pair.Source.Name != "" && pair.Source.Name != pair.Source.ID {
				pairsBySource[pair.Source.Name] = append(pairsBySource[pair.Source.Name], pair)
			}
		}
	}
	nicKeys := make([]string, len(enis))
	for i, eni := range enis {
		if eni.SubnetId != nil {
			nicKeys[i] = *eni.SubnetId
		}
	}
	return nicKeys, pairsBySource
}
