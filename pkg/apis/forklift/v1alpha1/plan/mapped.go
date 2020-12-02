package plan

import "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/mapped"

//
// Mapped resources.
type Map struct {
	// Networks.
	Networks []mapped.NetworkPair `json:"networks,omitempty"`
	// Datastores.
	Datastores []mapped.StoragePair `json:"datastores,omitempty"`
}

//
// Find network map for source ID.
func (r *Map) FindNetwork(networkID string) (pair mapped.NetworkPair, found bool) {
	for _, pair = range r.Networks {
		if pair.Source.ID == networkID {
			found = true
			break
		}
	}

	return
}

//
// Find storage map for source ID.
func (r *Map) FindStorage(storageID string) (pair mapped.StoragePair, found bool) {
	for _, pair = range r.Datastores {
		if pair.Source.ID == storageID {
			found = true
			break
		}
	}

	return
}
