package mapped

import "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1/ref"

//
// Mapped network destination.
type DestinationNetwork struct {
	// The network type.
	// +kubebuilder:validation:Enum=pod;multus
	Type string `json:"type"`
	// The namespace (multus only).
	Namespace string `json:"namespace,omitempty"`
	// The name.
	Name string `json:"name,omitempty"`
}

//
// Mapped network.
type NetworkPair struct {
	// Source network.
	Source ref.Ref `json:"source"`
	// Destination network.
	Destination DestinationNetwork `json:"destination"`
}
