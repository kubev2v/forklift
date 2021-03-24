package ocp

import (
	"github.com/konveyor/controller/pkg/inventory/container"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
)

//
// Routes
const (
	Root = base.ProvidersRoot + "/" + api.OpenShift
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("web")
	Log = &log
}

//
// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
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
		&VMHandler{
			Handler: Handler{
				base.Handler{Container: container},
			},
		},
	}
}
