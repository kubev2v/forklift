package ocp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
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
		h.watch(ctx)
		return
	}
	db := h.Collector.DB()
	list := []model.PersistentVolumeClaim{}
	err = db.List(&list, h.ListOptions(ctx))
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
	m := &model.PersistentVolumeClaim{
		Base: model.Base{
			UID: ctx.Param(PersistentVolumeClaimParam),
		},
	}
	db := h.Collector.DB()
	err = db.Get(m)
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
	r := &PersistentVolumeClaim{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h PersistentVolumeClaimHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.PersistentVolumeClaim{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.PersistentVolumeClaim)
			sc := &PersistentVolumeClaim{}
			sc.With(m)
			sc.Link(h.Provider)
			r = sc
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
