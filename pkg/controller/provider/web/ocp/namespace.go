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
	NamespacesRoot = ProviderRoot + "/namespaces"
	NamespaceRoot  = NamespacesRoot + "/:" + NsParam
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
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h NamespaceHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Namespace{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
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
			PK: ctx.Param(NsParam),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
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
			base.ProviderParam: string(p.UID),
			NsParam:            m.PK,
		})
}

//
// Watch.
func (h NamespaceHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Namespace{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Namespace)
			vm := &Namespace{}
			vm.With(m)
			vm.SelfLink = h.Link(h.Provider, m)
			r = vm
			return
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

//
// REST Resource.
type Namespace struct {
	Resource
	Object core.Namespace `json:"object"`
}

//
// Set fields with the specified object.
func (r *Namespace) With(m *model.Namespace) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

//
// As content.
func (r *Namespace) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
