package openstack

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
)

// Routes
const (
	Root = base.ProvidersRoot + "/" + string(api.OpenStack)
)

// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&RegionHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&ProjectHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&ImageHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&FlavorHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&VMHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&SnapshotHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&VolumeHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&VolumeTypeHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&NetworkHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&SubnetHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
		&TreeHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&WorkloadHandler{
			Handler{
				base.Handler{Container: container},
			},
		},
	}
}
