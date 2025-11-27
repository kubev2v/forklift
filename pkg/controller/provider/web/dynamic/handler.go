package dynamic

import (
	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libcontainer "github.com/kubev2v/forklift/pkg/lib/inventory/container"
)

// Routes
const (
	// ProvidersRoot for listing dynamic providers of a type: /providers/:type
	ProvidersRoot = base.ProvidersRoot + "/:type"
	// ProxyRoot for dynamic provider operations: /providers/:type/:provider
	ProxyRoot = ProvidersRoot + "/:provider"
)

// Handler handles requests for dynamic providers
type Handler struct {
	base.Handler
	registry *ProviderRegistry
}

// NewHandler creates a new dynamic provider handler
func NewHandler(container *libcontainer.Container, registry *ProviderRegistry) *Handler {
	return &Handler{
		Handler: base.Handler{
			Container: container,
		},
		registry: registry,
	}
}

// AddRoutes registers the dynamic provider routes
func (h *Handler) AddRoutes(e *gin.Engine) {
	// List providers of a specific dynamic type
	e.GET(ProvidersRoot, h.List)
	e.GET(ProvidersRoot+"/", h.List)

	// Catch-all for dynamic providers
	// Handles all requests including /refresh (handled inside Proxy method)
	// Must be registered AFTER static provider routes
	e.Any(ProxyRoot+"/*path", h.Proxy)
}
