package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	cnv "kubevirt.io/api/core/v1"
)

// Routes.
const (
	KubeVirtParam = "kubevirt"
	KubeVirtsRoot = ProviderRoot + "/kubevirts"
	KubeVirtRoot  = KubeVirtsRoot + "/:" + KubeVirtParam
)

// KubeVirt handler.
type KubeVirtHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *KubeVirtHandler) AddRoutes(e *gin.Engine) {
	e.GET(KubeVirtsRoot, h.List)
	e.GET(KubeVirtsRoot+"/", h.List)
	e.GET(KubeVirtRoot, h.Get)
}

// List resources in a REST collection.
func (h KubeVirtHandler) List(ctx *gin.Context) {
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
	kvs, err := h.KubeVirts(ctx, h.Provider)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	content := []interface{}{}
	for _, m := range kvs {
		r := &KubeVirt{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h KubeVirtHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	kvs, err := h.KubeVirts(ctx, h.Provider)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range kvs {
		if ctx.Param(KubeVirtParam) == m.UID {
			r := &KubeVirt{}
			r.With(&m)
			r.Link(h.Provider)
			content := r.Content(model.MaxDetail)

			ctx.JSON(http.StatusOK, content)
			return
		}
	}
	ctx.Status(http.StatusNotFound)
}

// REST Resource.
type KubeVirt struct {
	Resource
	Object cnv.KubeVirt `json:"object"`
}

// Set fields with the specified object.
func (r *KubeVirt) With(m *model.KubeVirt) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *KubeVirt) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		KubeVirtRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			KubeVirtParam:      r.UID,
		})
}

// As content.
func (r *KubeVirt) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
