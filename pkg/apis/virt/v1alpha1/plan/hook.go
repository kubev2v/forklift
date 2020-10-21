package plan

import core "k8s.io/api/core/v1"

//
// Plan hook.
type Hook struct {
	// Pre-migration hook.
	Before *core.ObjectReference `json:"before,omitempty"`
	// Post-migration hook.
	After *core.ObjectReference `json:"after,omitempty"`
}
