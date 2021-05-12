package ovirt

import (
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
)

//
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
	// Object description.
	Description string `json:"description"`
	// Self link.
	SelfLink string `json:"selfLink"`
}

//
// Build the resource using the model.
func (r *Resource) With(m *model.Base) {
	r.ID = m.ID
	r.Revision = m.Revision
	r.Name = m.Name
	r.Description = m.Description
}
