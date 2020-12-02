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
		&NamespaceHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&StorageClassHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
		&NetworkAttachmentDefinitionHandler{
			Handler: base.Handler{
				Container: container,
			},
		},
	}
}
