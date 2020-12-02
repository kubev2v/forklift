package ocp

import (
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
)

//
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
}

//
// Populate the fields with the specified object.
func (r *Resource) With(m *model.Base) {
	r.UID = m.UID
	r.Version = m.Version
	r.Namespace = m.Namespace
	r.Name = m.Name
}
