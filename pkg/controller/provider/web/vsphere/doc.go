package web

import (
	"github.com/konveyor/controller/pkg/inventory/container"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
)

//
// Routes
const (
	Root = base.ProviderRoot + "/" + api.VSphere
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
		&libweb.SchemaHandler{},
		&base.ProviderHandler{
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
