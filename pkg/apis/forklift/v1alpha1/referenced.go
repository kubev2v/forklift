package v1alpha1

import core "k8s.io/api/core/v1"

//
// Referenced resources.
// Holds resources fetched during validation.
// +k8s:deepcopy-gen=false
type Referenced struct {
	// Provider.
	Provider struct {
		Source      *Provider
		Destination *Provider
	}
	// Secret.
	Secret *core.Secret
	// Plan
	Plan *Plan
	// Map
	Map struct {
		// Network
		Network *NetworkMap
		// Storage
		Storage *StorageMap
	}
}

func (in *Referenced) DeepCopyInto(*Referenced) {
}

func (in *Referenced) DeepCopy() *Referenced {
	return in
}
