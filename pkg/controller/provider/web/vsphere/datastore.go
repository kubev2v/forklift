package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	DatastoresRoot = Root + "/datastores"
	DatastoreRoot  = DatastoresRoot + "/:datastore"
)

//
// Datastore handler.
type DatastoreHandler struct {
	base.Handler
}

//
// Add routes to the `gin` router.
func (h *DatastoreHandler) AddRoutes(e *gin.Engine) {
	e.GET(DatastoresRoot, h.List)
	e.GET(DatastoresRoot+"/", h.List)
	e.GET(DatastoreRoot, h.Get)
}

//
// List resources in a REST collection.
func (h DatastoreHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	selector := &model.Datastore{}
	options := libmodel.ListOptions{
		Page: &h.Page,
	}
	list := []model.Datastore{}
	err := db.List(selector, options, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Datastore{}
		r.With(&m)
		obj := r.Object(h.Detail)
		content = append(content, obj)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h DatastoreHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Datastore{
		Base: model.Base{
			ID: ctx.Param("datastore"),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &Datastore{}
	r.With(m)

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Datastore struct {
	base.Resource
	Type            string `json:"type"`
	Capacity        int64  `json:"capacity"`
	Free            int64  `json:"free"`
	MaintenanceMode string `json:"maintenance"`
}

//
// Build the resource using the model.
func (r *Datastore) With(m *model.Datastore) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Capacity = m.Capacity
	r.Free = m.Free
	r.MaintenanceMode = m.MaintenanceMode
}

//
// Render.
func (r *Datastore) Object(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
