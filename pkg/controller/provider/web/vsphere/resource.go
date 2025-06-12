package vsphere

import (
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
)

// REST Resource.
type Resource struct {
	// Object ID.
	ID string `json:"id"`
	// Variant
	Variant string `json:"variant,omitempty"`
	// Parent.
	Parent model.Ref `json:"parent"`
	// Path
	Path string `json:"path,omitempty"`
	// Revision
	Revision int64 `json:"revision"`
	// Object name.
	Name string `json:"name"`
	// Self link.
	SelfLink string `json:"selfLink"`
}

// Build the resource using the model.
func (r *Resource) With(m *model.Base) {
	r.ID = m.ID
	r.Variant = m.Variant
	r.Parent = m.Parent
	r.Revision = m.Revision
	r.Name = m.Name
}
