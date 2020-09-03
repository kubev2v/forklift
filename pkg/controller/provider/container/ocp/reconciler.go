package ocp

import (
	libocp "github.com/konveyor/controller/pkg/inventory/container/ocp"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	core "k8s.io/api/core/v1"
)

type Reconciler struct {
	*libocp.Reconciler
}

//
// New reconciler.
func New(db libmodel.DB, provider *api.Provider, secret *core.Secret) *libocp.Reconciler {
	return libocp.New(
		db,
		provider,
		secret,
		Log,
		&NetworkAttachmentDefinition{},
		&StorageClass{})
}
