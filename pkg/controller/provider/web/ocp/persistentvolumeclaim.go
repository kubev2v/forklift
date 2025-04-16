package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	core "k8s.io/api/core/v1"
)

// Routes.
const (
	PersistentVolumeClaimParam = "pvc"
	PersistentVolumeClaimsRoot = ProviderRoot + "/persistentvolumeclaims"
	PersistentVolumeClaimRoot  = PersistentVolumeClaimsRoot + "/:" + PersistentVolumeClaimParam
)

// PersistentVolumeClaim handler.
type PersistentVolumeClaimHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *PersistentVolumeClaimHandler) AddRoutes(e *gin.Engine) {
	e.GET(PersistentVolumeClaimsRoot, h.List)
	e.GET(PersistentVolumeClaimsRoot+"/", h.List)
	e.GET(PersistentVolumeClaimRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h PersistentVolumeClaimHandler) List(ctx *gin.Context) {
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
	pvcs, err := h.PersistentVolumeClaims(ctx, h.Provider)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range pvcs {
		r := &PersistentVolumeClaim{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h PersistentVolumeClaimHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	pvcs, err := h.PersistentVolumeClaims(ctx, h.Provider)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range pvcs {
		if m.UID == ctx.Param(PersistentVolumeClaimParam) {
			r := &PersistentVolumeClaim{}
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
type PersistentVolumeClaim struct {
	Resource
	Object core.PersistentVolumeClaim `json:"object"`
}

// Set fields with the specified object.
func (r *PersistentVolumeClaim) With(m *model.PersistentVolumeClaim) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *PersistentVolumeClaim) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		PersistentVolumeClaimRoot,
		base.Params{
			base.ProviderParam:         string(p.UID),
			PersistentVolumeClaimParam: r.UID,
		})
}

// As content.
func (r *PersistentVolumeClaim) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
