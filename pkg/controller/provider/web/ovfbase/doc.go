// Package ovfbase provides shared web handlers for OVF-based providers (OVA and HyperV).
// Both providers use the same data model (ova/model.go) and share identical handler logic.
// The only differences are the URL paths and provider type checks.
package ovfbase

import (
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/lib/inventory/container"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
)

// Config holds provider-specific configuration for handlers.
type Config struct {
	// ProviderType is the API provider type (api.Ova or api.HyperV)
	ProviderType api.ProviderType
	// Root is the base URL path for this provider's routes
	Root string
}

// Routes derived from Root
func (c *Config) ProvidersRoot() string {
	return c.Root
}

func (c *Config) ProviderRoot() string {
	return c.Root + "/:" + base.ProviderParam
}

// Build all handlers for an OVF-based provider.
func Handlers(container *container.Container, cfg Config) []libweb.RequestHandler {
	return []libweb.RequestHandler{
		&ProviderHandler{
			Handler: base.Handler{
				Container: container,
			},
			Config: cfg,
		},
		&TreeHandler{
			Handler: Handler{
				Handler: base.Handler{Container: container},
				Config:  cfg,
			},
		},
		&DiskHandler{
			Handler: Handler{
				Handler: base.Handler{Container: container},
				Config:  cfg,
			},
		},
		&NetworkHandler{
			Handler: Handler{
				Handler: base.Handler{Container: container},
				Config:  cfg,
			},
		},
		&VMHandler{
			Handler: Handler{
				Handler: base.Handler{Container: container},
				Config:  cfg,
			},
		},
		&WorkloadHandler{
			Handler: Handler{
				Handler: base.Handler{Container: container},
				Config:  cfg,
			},
		},
		&StorageHandler{
			Handler: Handler{
				Handler: base.Handler{Container: container},
				Config:  cfg,
			},
		},
	}
}
