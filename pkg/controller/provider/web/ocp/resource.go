package ocp

import (
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
)

// REST Resource.
type Resource struct {
	// k8s UID.
	UID string `json:"uid"`
	// k8s resource version.
	Version string `json:"version"`
	// k8s namespace.
	Namespace string `json:"namespace"`
	// k8s name.
	Name string `json:"name"`
	// self link.
	SelfLink string `json:"selfLink"`
	// self path.
	Path string `json:"path,omitempty"`

	// forklift ID, for compatability with providers using ID instead of UID
	ID string `json:"id,omitempty"`
}

// Populate the fields with the specified object.
func (r *Resource) With(m *model.Base) {
	r.UID = m.UID
	r.Version = m.Version
	r.Namespace = m.Namespace
	r.Name = m.Name

	r.ID = m.UID
}
