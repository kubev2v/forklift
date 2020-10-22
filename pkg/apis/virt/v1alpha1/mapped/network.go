package mapped

//
// Mapped network destination.
type DestinationNetwork struct {
	// The network type (pod|multus)
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
	Source SourceObject `json:"source"`
	// Destination network.
	Destination DestinationNetwork `json:"destination"`
}
