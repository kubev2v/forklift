package web

import (
	"github.com/gin-gonic/gin"
	libweb "github.com/konveyor/controller/pkg/inventory/web"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Ns handler.
type NsHandler struct {
	base.Handler
}

//
// Add routes to the `gin` router.
func (h *NsHandler) AddRoutes(e *gin.Engine) {
	e.GET(libweb.NsCollection, h.List)
	e.GET(libweb.NsCollection+"/", h.List)
	e.GET(libweb.NsCollection+"/"+":"+base.NsParam, h.Get)
}

//
// List resources in a REST collection.
func (h NsHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	set := map[string]bool{}
	list := h.Container.List()
	for _, reconciler := range list {
		if p, cast := reconciler.Owner().(*api.Provider); cast {
			if p.Type() != api.VSphere {
				continue
			}
			if reconciler, found := h.Container.Get(p); found {
				h.Reconciler = reconciler
			} else {
				continue
			}

			set[p.Namespace] = true
		}
	}
	content := []Namespace{}
	for ns := range set {
		content = append(
			content, Namespace{
				SelfLink: h.Link(ns),
				Name:     ns,
			})
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h NsHandler) Get(ctx *gin.Context) {
	name := ctx.Param(base.NsParam)
	content := Namespace{
		SelfLink: h.Link(name),
		Name:     name,
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h NsHandler) Link(ns string) string {
	return h.Handler.Link(
		base.Root,
		base.Params{
			base.NsParam: ns,
		})
}

//
// REST Resource.
type Namespace struct {
	SelfLink string `json:"selfLink"`
	Name     string `json:"name"`
}
