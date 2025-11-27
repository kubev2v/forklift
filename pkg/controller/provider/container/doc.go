package container

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/container/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/container/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/container/ova"
	"github.com/kubev2v/forklift/pkg/controller/provider/container/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/container/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/dynamic"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	core "k8s.io/api/core/v1"
)

// Build creates a collector for the given provider.
// For static provider types, it creates type-specific collectors.
// For dynamic provider types, it creates a dynamic collector that proxies to the provider server.
func Build(
	db libmodel.DB,
	provider *api.Provider,
	secret *core.Secret) libcontainer.Collector {

	// Check if this is a dynamic provider type
	if dynamic.Registry.IsDynamic(string(provider.Type())) {
		return dynamic.New(provider, db)
	}

	// Static provider types with built-in collectors
	switch provider.Type() {
	case api.OpenShift:
		return ocp.New(nil, provider, secret)
	case api.VSphere:
		return vsphere.New(db, provider, secret)
	case api.OVirt:
		return ovirt.New(db, provider, secret)
	case api.OpenStack:
		return openstack.New(db, provider, secret)
	case api.Ova:
		return ova.New(db, provider, secret)
	}

	return nil
}
