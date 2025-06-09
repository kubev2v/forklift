package container

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/ova"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/vsphere"
	libcontainer "github.com/konveyor/forklift-controller/pkg/lib/inventory/container"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	core "k8s.io/api/core/v1"
)

// Build
func Build(
	db libmodel.DB,
	provider *api.Provider,
	secret *core.Secret) libcontainer.Collector {
	//
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
