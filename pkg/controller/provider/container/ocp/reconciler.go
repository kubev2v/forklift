package ocp

import (
	libcontainer "github.com/konveyor/controller/pkg/inventory/container"
	libocp "github.com/konveyor/controller/pkg/inventory/container/ocp"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	core "k8s.io/api/core/v1"
)

//
// New reconciler.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) libcontainer.Reconciler {
	return &Reconciler{
		Reconciler: libocp.New(
			db,
			provider,
			secret,
			&Namespace{},
			&NetworkAttachmentDefinition{},
			&StorageClass{},
			&VM{}),
	}
}

//
// OCP reconciler.
type Reconciler struct {
	*libocp.Reconciler
}

//
// Test api-server URL and token.
func (r *Reconciler) Test() error {
	// TODO: SAR check the token has access to kubevirt.
	return nil
}
