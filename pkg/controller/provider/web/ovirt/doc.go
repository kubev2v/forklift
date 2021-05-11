package ovirt

import (
	"github.com/konveyor/controller/pkg/inventory/container"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
)

//
// Routes
const (
	Root = base.ProvidersRoot + "/" + api.OVirt
)

//
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
		&VNICProfileHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&StorageDomainHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
	}
}
