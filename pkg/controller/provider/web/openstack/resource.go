package openstack

import (
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/openstack"
)

// REST Resource.
type Resource struct {
	// Object ID.
	ID string `json:"id"`
	// Revision
	Revision int64 `json:"revision"`
	// Path
	Path string `json:"path,omitempty"`
	// Object name.
	Name string `json:"name"`
	// Self link.
	SelfLink string `json:"selfLink"`
}

// Build the resource using the model.
func (r *Resource) With(m *model.Base) {
	r.ID = m.ID
	r.Name = m.Name
	r.Revision = m.Revision
}
