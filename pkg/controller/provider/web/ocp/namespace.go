package ocp

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	core "k8s.io/api/core/v1"
	"net/http"
)

//
// Routes.
const (
	Ns2Param       = "ns2"
	NamespacesRoot = ProviderRoot + "/namespaces"
	NamespaceRoot  = NamespacesRoot + "/:" + Ns2Param
)

//
// Namespace handler.
type NamespaceHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *NamespaceHandler) AddRoutes(e *gin.Engine) {
	e.GET(NamespacesRoot, h.List)
	e.GET(NamespacesRoot+"/", h.List)
	e.GET(NamespaceRoot, h.Get)
}

//
// List resources in a REST collection.
func (h NamespaceHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Namespace{}
	err := db.List(
		&list,
		libmodel.ListOptions{
			Page: &h.Page,
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Namespace{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h NamespaceHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Namespace{
		Base: model.Base{
			Name: ctx.Param(Ns2Param),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &Namespace{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h NamespaceHandler) Link(p *api.Provider, m *model.Namespace) string {
	return h.Handler.Link(
		NamespaceRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			Ns2Param:           m.Name,
		})
}

//
// REST Resource.
type Namespace struct {
	Resource
	Object interface{} `json:"object"`
}

//
// Set fields with the specified object.
func (r *Namespace) With(m *model.Namespace) {
	r.Resource.With(&m.Base)
	r.Object = m.DecodeObject(&core.Namespace{})
}

//
// As content.
func (r *Namespace) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
