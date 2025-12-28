package ovfbase

import (
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
)

// REST resource.
type Resource struct {
	// Object ID.
	ID string `json:"id"`
	// Variant
	Variant string `json:"variant,omitempty"`
	// Object name.
	Name string `json:"name"`
	// Self link.
	SelfLink string `json:"selfLink"`
	// Path
	Path string `json:"path,omitempty"`
}

// Build the resource using the model.
func (r *Resource) With(m *model.Base) {
	r.ID = m.ID
	r.Name = m.Name
}
