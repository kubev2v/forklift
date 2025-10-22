package vsphere

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	DatacenterParam      = "datacenter"
	DatacenterCollection = "datacenters"
	DatacentersRoot      = ProviderRoot + "/" + DatacenterCollection
	DatacenterRoot       = DatacentersRoot + "/:" + DatacenterParam
)

// Datacenter handler.
type DatacenterHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *DatacenterHandler) AddRoutes(e *gin.Engine) {
	e.GET(DatacentersRoot, h.List)
	e.GET(DatacentersRoot+"/", h.List)
	e.GET(DatacenterRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DatacenterHandler) List(ctx *gin.Context) {
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
	list := []model.Datacenter{}
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
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &Datacenter{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h DatacenterHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Datacenter{
		Base: model.Base{
			ID: ctx.Param(DatacenterParam),
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
	pb := PathBuilder{DB: db}
	r := &Datacenter{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *DatacenterHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Datacenter{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Datacenter)
			dc := &Datacenter{}
			dc.With(m)
			dc.Link(h.Provider)
			dc.Path = pb.Path(m)
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

// REST Resource.
type Datacenter struct {
	Resource
	Datastores model.Ref `json:"datastores"`
	Networks   model.Ref `json:"networks"`
	Clusters   model.Ref `json:"clusters"`
	VMs        model.Ref `json:"vms"`
}

// Build the resource using the model.
func (r *Datacenter) With(m *model.Datacenter) {
	r.Resource.With(&m.Base)
	r.Datastores = m.Datastores
	r.Networks = m.Networks
	r.Clusters = m.Clusters
	r.VMs = m.Vms
}

// Build self link (URI).
func (r *Datacenter) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		DatacenterRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DatacenterParam:    r.ID,
		})
}

// As content.
func (r *Datacenter) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
