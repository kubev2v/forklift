package web

import (
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/openstack"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ova"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	"github.com/konveyor/forklift-controller/pkg/lib/inventory/container"
	libweb "github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
)

// All handlers.
func All(container *container.Container) (all []libweb.RequestHandler) {
	all = []libweb.RequestHandler{
		&libweb.SchemaHandler{},
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
	}
	all = append(
		all,
		ocp.Handlers(container)...)
	all = append(
		all,
		vsphere.Handlers(container)...)
	all = append(
		all,
		ovirt.Handlers(container)...)
	all = append(
		all,
		openstack.Handlers(container)...)
	all = append(
		all,
		ova.Handlers(container)...)
	return
}
