package ocp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	cdi "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// Routes.
const (
	DataVolumeParam = "dv"
	DataVolumesRoot = ProviderRoot + "/datavolumes"
	DataVolumeRoot  = DataVolumesRoot + "/:" + DataVolumeParam
)

// DataVolume handler.
type DataVolumeHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *DataVolumeHandler) AddRoutes(e *gin.Engine) {
	e.GET(DataVolumesRoot, h.List)
	e.GET(DataVolumesRoot+"/", h.List)
	e.GET(DataVolumeRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DataVolumeHandler) List(ctx *gin.Context) {
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
	list := []model.DataVolume{}
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
		r := &DataVolume{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h DataVolumeHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.DataVolume{
		Base: model.Base{
			UID: ctx.Param(DataVolumeParam),
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
	r := &DataVolume{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h DataVolumeHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.DataVolume{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.DataVolume)
			sc := &DataVolume{}
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
type DataVolume struct {
	Resource
	Object cdi.DataVolume `json:"object"`
}

// Set fields with the specified object.
func (r *DataVolume) With(m *model.DataVolume) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *DataVolume) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		DataVolumeRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DataVolumeParam:    r.UID,
		})
}

// As content.
func (r *DataVolume) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
