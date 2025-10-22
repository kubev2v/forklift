package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Routes.
const (
	NamespacesRoot = ProviderRoot + "/namespaces"
	NamespaceRoot  = NamespacesRoot + "/:" + NsParam
)

// Namespace handler.
type NamespaceHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *NamespaceHandler) AddRoutes(e *gin.Engine) {
	e.GET(NamespacesRoot, h.List)
	e.GET(NamespacesRoot+"/", h.List)
	e.GET(NamespaceRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h NamespaceHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusNotImplemented)
		return
	}

	namespaces, err := h.Namespaces(ctx, h.Provider)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	content := []interface{}{}
	for _, m := range namespaces {
		r := Namespace{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h NamespaceHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	namespaces, err := h.Namespaces(ctx, h.Provider)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	uid := types.UID(ctx.Param(NsParam))
	for _, ns := range namespaces {
		if ns.Object.ObjectMeta.UID == uid {
			r := Namespace{}
			r.With(&ns)
			r.Link(h.Provider)
			content := r.Content(model.MaxDetail)
			ctx.JSON(http.StatusOK, content)
			return
		}
	}

	ctx.Status(http.StatusNotFound)
}

// REST Resource.
type Namespace struct {
	Resource
	Object core.Namespace `json:"object"`
}

// Set fields with the specified object.
func (r *Namespace) With(m *model.Namespace) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *Namespace) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		NamespaceRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			NsParam:            r.UID,
		})
}

// As content.
func (r *Namespace) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
