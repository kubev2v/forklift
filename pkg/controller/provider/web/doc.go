package web

import (
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/openstack"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ova"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/vsphere"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	ec2web "github.com/kubev2v/forklift/pkg/provider/ec2/inventory/web"
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
	all = append(
		all,
		ec2web.Handlers(container)...)
	all = append(
		all,
		hyperv.Handlers(container)...)
	return
}
