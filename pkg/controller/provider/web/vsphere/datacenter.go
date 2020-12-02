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
	base.Handler
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
// Prepare to handle the request.
func (h *DatacenterHandler) Prepare(ctx *gin.Context) int {
	status := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return status
	}
	id := ctx.Param(DatacenterParam)
	if id != "" {
		m := &model.Datacenter{
			Base: model.Base{
				ID: id,
			},
		}
		db := h.Reconciler.DB()
		err := db.Get(m)
		if errors.Is(err, model.NotFound) {
			return http.StatusNotFound
		}
		if err != nil {
			Log.Trace(err)
			return http.StatusInternalServerError
		}

		h.datacenter = m
	}

	return http.StatusOK
}

//
// List resources in a REST collection.
func (h DatacenterHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.Datacenter{}
	err := db.List(
		&list,
		libmodel.ListOptions{
			Page: &h.Page,
		})
	if err != nil {
		Log.Trace(err)
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
	r := &Datacenter{}
	r.With(h.datacenter)
	r.SelfLink = h.Link(h.Provider, h.datacenter)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h DatacenterHandler) Link(p *api.Provider, m *model.Datacenter) string {
	return h.Handler.Link(
		DatacenterRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			DatacenterParam:    m.ID,
		})
}

//
// REST Resource.
type Datacenter struct {
	Resource
}

//
// Build the resource using the model.
func (r *Datacenter) With(m *model.Datacenter) {
	r.Resource.With(&m.Base)
}

//
// As content.
func (r *Datacenter) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
