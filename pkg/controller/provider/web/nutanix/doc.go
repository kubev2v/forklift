package nutanix

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
)

// Routes
const (
	Root = base.ProvidersRoot + "/" + string(api.Nutanix)
)

// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
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
		&NetworkHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&StorageContainerHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&ImageHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&VMHandler{
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
