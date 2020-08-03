package base

import (
	"github.com/gin-gonic/gin"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"net/http"
)

//
// Routes.
const (
	ProvidersRoot = Root + "/providers"
	ProviderRoot  = ProvidersRoot + "/:provider"
)

//
// Provider handler.
type ProviderHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProvidersRoot, h.List)
	e.GET(ProvidersRoot+"/", h.List)
	e.GET(ProviderRoot, h.Get)
}

//
// List resources in a REST collection.
func (h ProviderHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	list := h.Container.List()
	content := []interface{}{}
	for _, reconciler := range list {
		if p, cast := reconciler.Owner().(*api.Provider); cast {
			if !h.Detail {
				content = append(
					content,
					map[string]string{
						"namespace": p.Namespace,
						"name":      p.Name,
					})
			} else {
				content = append(content, p)
			}
		}
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h ProviderHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}

	ctx.JSON(http.StatusOK, h.Provider)
}

//
// REST Resource.
type Provider = api.Provider
