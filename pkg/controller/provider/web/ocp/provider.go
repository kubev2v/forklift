package ocp

import (
	"github.com/gin-gonic/gin"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/virt-controller/pkg/apis/virt/v1alpha1"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	ProviderParam      = base.ProviderParam
	ProviderCollection = "providers"
	ProvidersRoot      = libweb.Root + "/" + ProviderCollection
	ProviderRoot       = ProvidersRoot + "/:" + ProviderParam
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
			r := Provider{}
			r.With(p)
			r.SelfLink = h.Link(p)
			content = append(content, r.Content(h.Detail))
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
	r := Provider{}
	r.With(h.Provider)
	r.SelfLink = h.Link(h.Provider)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h ProviderHandler) Link(m *api.Provider) string {
	return h.Handler.Link(
		ProviderRoot,
		base.Params{
			base.NsParam:  m.Namespace,
			ProviderParam: m.Name,
		})
}

//
// REST Resource.
type Provider struct {
	Resource
	Object interface{} `json:"object"`
}

//
// Set fields with the specified object.
func (r *Provider) With(m *api.Provider) {
	r.Namespace = m.Namespace
	r.Name = m.Name
	r.Object = m
}

//
// As content.
func (r *Provider) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
