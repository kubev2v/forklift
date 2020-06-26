package web

import (
	"github.com/konveyor/controller/pkg/inventory/container"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	"github.com/konveyor/controller/pkg/logging"
)

//
// Shared logger.
var Log *logging.Logger

func init() {
	log := logging.WithName("model")
	Log = &log
}

//
// Build all handlers.
func All(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&libweb.SchemaHandler{},
		&FolderHandler{
			Base: Base{
				Container: container,
			},
		},
		&DatacenterHandler{
			Base: Base{
				Container: container,
			},
		},
		&ClusterHandler{
			Base: Base{
				Container: container,
			},
		},
		&HostHandler{
			Base: Base{
				Container: container,
			},
		},
		&NetworkHandler{
			Base: Base{
				Container: container,
			},
		},
		&DatastoreHandler{
			Base: Base{
				Container: container,
			},
		},
		&VMHandler{
			Base: Base{
				Container: container,
			},
		},
	}
}
