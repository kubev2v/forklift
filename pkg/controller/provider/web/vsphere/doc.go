package vsphere

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
	Root = base.ProvidersRoot + "/" + api.VSphere
)

type Handler = base.Handler

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
		&TreeHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&FolderHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&DatacenterHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&ClusterHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&HostHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&NetworkHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&DatastoreHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&VMHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
	}
}
