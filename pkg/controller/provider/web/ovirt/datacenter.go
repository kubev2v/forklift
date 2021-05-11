package ovirt

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	DataCenterParam      = "datacenter"
	DataCenterCollection = "datacenters"
	DataCentersRoot      = ProviderRoot + "/" + DataCenterCollection
	DataCenterRoot       = DataCentersRoot + "/:" + DataCenterParam
)

//
// DataCenter handler.
type DataCenterHandler struct {
	Handler
	// Selected DataCenter.
	datacenter *model.DataCenter
}

//
// Add routes to the `gin` router.
func (h *DataCenterHandler) AddRoutes(e *gin.Engine) {
	e.GET(DataCentersRoot, h.List)
	e.GET(DataCentersRoot+"/", h.List)
	e.GET(DataCenterRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DataCenterHandler) List(ctx *gin.Context) {
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
	list := []model.DataCenter{}
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
		r := &DataCenter{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h DataCenterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.DataCenter{
		Base: model.Base{
			ID: ctx.Param(DataCenterParam),
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
	r := &DataCenter{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h DataCenterHandler) Link(p *api.Provider, m *model.DataCenter) string {
	return h.Handler.Link(
		DataCenterRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DataCenterParam:    m.ID,
		})
}

//
// Watch.
func (h DataCenterHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.DataCenter{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.DataCenter)
			dc := &DataCenter{}
			dc.With(m)
			dc.SelfLink = h.Link(h.Provider, m)
			r = dc
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
type DataCenter struct {
	Resource
}

//
// Build the resource using the model.
func (r *DataCenter) With(m *model.DataCenter) {
	r.Resource.With(&m.Base)
}

//
// As content.
func (r *DataCenter) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
