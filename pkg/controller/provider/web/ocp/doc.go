package ocp

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
)

// Routes
const (
	Root = base.ProvidersRoot + "/" + string(api.OpenShift)
)

// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&NamespaceHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&StorageClassHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&NadHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&InstanceHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&ClusterInstanceHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&VMHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
		&TreeHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
	}
}
