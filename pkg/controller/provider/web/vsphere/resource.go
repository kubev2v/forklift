package vsphere

import (
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
)

//
// REST Resource.
type Resource struct {
	// Object ID.
	ID string `json:"id"`
	// Parent.
	Parent *model.Ref `json:"parent"`
	// Revision
	Revision int64 `json:"revision"`
	// Object name.
	Name string `json:"name"`
	// Self link.
	SelfLink string `json:"selfLink"`
}

//
// Build the resource using the model.
func (r *Resource) With(m *model.Base) {
	r.ID = m.ID
	r.Parent = (&model.Ref{}).With(m.Parent)
	r.Revision = m.Revision
	r.Name = m.Name
}
