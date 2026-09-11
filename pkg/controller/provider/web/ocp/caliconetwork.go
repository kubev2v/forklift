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
	CalicoNetworkParam = "caliconetwork"
	CalicoNetworksRoot = ProviderRoot + "/caliconetworks"
	CalicoNetworkRoot  = CalicoNetworksRoot + "/:" + CalicoNetworkParam
)

// CalicoNetwork handler.
type CalicoNetworkHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *CalicoNetworkHandler) AddRoutes(e *gin.Engine) {
	e.GET(CalicoNetworksRoot, h.List)
	e.GET(CalicoNetworksRoot+"/", h.List)
	e.GET(CalicoNetworkRoot, h.Get)
}

// List resources in a REST collection.
func (h CalicoNetworkHandler) List(ctx *gin.Context) {
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
	networks, err := h.CalicoNetworks(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range networks {
		r := &CalicoNetwork{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	h.Page.Slice(&content)

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h CalicoNetworkHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	networks, err := h.CalicoNetworks(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range networks {
		if m.UID == ctx.Param(CalicoNetworkParam) {
			r := &CalicoNetwork{}
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
type CalicoNetwork struct {
	Resource
	Object calico.Network `json:"object"`
}

// Set fields with the specified object.
func (r *CalicoNetwork) With(m *model.CalicoNetwork) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *CalicoNetwork) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		CalicoNetworkRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			CalicoNetworkParam: r.UID,
		})
}

// As content.
func (r *CalicoNetwork) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
