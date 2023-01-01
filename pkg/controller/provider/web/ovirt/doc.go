package ovirt

import (
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"github.com/konveyor/forklift-controller/pkg/lib/inventory/container"
	libweb "github.com/konveyor/forklift-controller/pkg/lib/inventory/web"
)

// Routes
const (
	Root = base.ProvidersRoot + "/" + string(api.OVirt)
)

// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&DataCenterHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&ClusterHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&HostHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&VMHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&NetworkHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&NICProfileHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&DiskProfileHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&StorageDomainHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&DiskHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&TreeHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&WorkloadHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
	}
}
