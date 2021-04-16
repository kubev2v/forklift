package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	DatacenterParam      = "datacenter"
	DatacenterCollection = "datacenters"
	DatacentersRoot      = ProviderRoot + "/" + DatacenterCollection
	DatacenterRoot       = DatacentersRoot + "/:" + DatacenterParam
)

//
// Datacenter handler.
type DatacenterHandler struct {
	Handler
	// Selected Datacenter.
	datacenter *model.Datacenter
}

//
// Add routes to the `gin` router.
func (h *DatacenterHandler) AddRoutes(e *gin.Engine) {
	e.GET(DatacentersRoot, h.List)
	e.GET(DatacentersRoot+"/", h.List)
	e.GET(DatacenterRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DatacenterHandler) List(ctx *gin.Context) {
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
	list := []model.Datacenter{}
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
		r := &Datacenter{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h DatacenterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Datacenter{
		Base: model.Base{
			ID: ctx.Param(DatacenterParam),
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
	r := &Datacenter{}
	r.With(m)
	r.Path, err = m.Path(db)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h DatacenterHandler) Link(p *api.Provider, m *model.Datacenter) string {
	return h.Handler.Link(
		DatacenterRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DatacenterParam:    m.ID,
		})
}

//
// Watch.
func (h DatacenterHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Datacenter{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Datacenter)
			dc := &Datacenter{}
			dc.With(m)
			dc.SelfLink = h.Link(h.Provider, m)
			dc.Path, _ = m.Path(db)
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
type Datacenter struct {
	Resource
	Datastores model.Ref `json:"datastores"`
	Networks   model.Ref `json:"networks"`
	Clusters   model.Ref `json:"clusters"`
	VMs        model.Ref `json:"vms"`
}

//
// Build the resource using the model.
func (r *Datacenter) With(m *model.Datacenter) {
	r.Resource.With(&m.Base)
	r.Datastores = m.Datastores
	r.Networks = m.Networks
	r.Clusters = m.Clusters
	r.VMs = m.Vms
}

//
// As content.
func (r *Datacenter) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
