package hyperv

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
)

var log = logging.WithName("web|hyperv")

// Routes
const (
	Root = base.ProvidersRoot + "/" + string(api.HyperV)
)

var DefaultConfig = Config{
	ProviderType: api.HyperV,
	Root:         Root,
}

// Build all handlers.
func Handlers(container *container.Container) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
			},
			Config: DefaultConfig,
		},
		&VMHandler{
			Handler: Handler{
				Handler: base.Handler{
					Container: container,
				},
			},
		},
		&NetworkHandler{
			Handler: Handler{
				Handler: base.Handler{
					Container: container,
				},
			},
		},
		&StorageHandler{
			Handler: Handler{
				Handler: base.Handler{
					Container: container,
				},
			},
		},
		&DiskHandler{
			Handler: Handler{
				Handler: base.Handler{
					Container: container,
				},
			},
		},
		&TreeHandler{
			Handler: Handler{
				Handler: base.Handler{
					Container: container,
				},
			},
		},
		&WorkloadHandler{
			Handler: Handler{
				Handler: base.Handler{
					Container: container,
				},
			},
			Config: DefaultConfig,
		},
	}
}
