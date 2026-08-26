package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	calico "github.com/kubev2v/forklift/pkg/lib/client/calico"
)

// Routes.
const (
	CalicoIPPoolParam = "calicoippool"
	CalicoIPPoolsRoot = ProviderRoot + "/calicoippools"
	CalicoIPPoolRoot  = CalicoIPPoolsRoot + "/:" + CalicoIPPoolParam
)

// CalicoIPPool handler.
type CalicoIPPoolHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *CalicoIPPoolHandler) AddRoutes(e *gin.Engine) {
	e.GET(CalicoIPPoolsRoot, h.List)
	e.GET(CalicoIPPoolsRoot+"/", h.List)
	e.GET(CalicoIPPoolRoot, h.Get)
}

// List resources in a REST collection.
func (h CalicoIPPoolHandler) List(ctx *gin.Context) {
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
	pools, err := h.CalicoIPPools(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range pools {
		r := &CalicoIPPool{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h CalicoIPPoolHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	pools, err := h.CalicoIPPools(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range pools {
		if m.UID == ctx.Param(CalicoIPPoolParam) {
			r := &CalicoIPPool{}
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
type CalicoIPPool struct {
	Resource
	Object calico.IPPool `json:"object"`
}

// Set fields with the specified object.
func (r *CalicoIPPool) With(m *model.CalicoIPPool) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *CalicoIPPool) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		CalicoIPPoolRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			CalicoIPPoolParam:  r.UID,
		})
}

// As content.
func (r *CalicoIPPool) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
