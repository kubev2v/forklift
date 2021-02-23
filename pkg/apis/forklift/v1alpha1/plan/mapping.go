package plan

import core "k8s.io/api/core/v1"

//
// Maps.
type Map struct {
	// Network.
	Network core.ObjectReference `json:"network" ref:"NetworkMap"`
	// Storage.
	Storage core.ObjectReference `json:"storage" ref:"StorageMap"`
}
