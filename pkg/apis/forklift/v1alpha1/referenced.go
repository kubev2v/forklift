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
	// Hooks.
	Hooks []*Hook
}

//
// Find hook by ref.
func (in *Referenced) FindHook(ref core.ObjectReference) (found bool, hook *Hook) {
	for _, hook = range in.Hooks {
		if hook.Namespace == ref.Namespace && hook.Name == ref.Name {
			found = true
			break
		}
	}

	return
}

func (in *Referenced) DeepCopyInto(*Referenced) {
}

func (in *Referenced) DeepCopy() *Referenced {
	return in
}
