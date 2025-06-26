package provider

import core "k8s.io/api/core/v1"

// Referenced Provider pair.
type Pair struct {
	// Source.
	Source core.ObjectReference `json:"source" ref:"Provider"`
	// Destination.
	Destination core.ObjectReference `json:"destination" ref:"Provider"`
}
