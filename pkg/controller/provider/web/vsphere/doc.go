package vsphere

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/settings"
)

// Routes
const (
	Root = base.ProvidersRoot + "/" + string(api.VSphere)
)

// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	handlers := []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&TreeHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&FolderHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&DatacenterHandler{
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
		&NetworkHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&DatastoreHandler{
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

	if settings.Settings.OpenShift {
		handlers = append(
			handlers,
			&VddkHandler{
				Handler: base.Handler{
					Container: container,
				},
			},
		)
	}

	return handlers
}
