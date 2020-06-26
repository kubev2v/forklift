package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"net/http"
)

const (
	DatastoresRoot = Root + "/datastores"
	DatastoreRoot  = DatastoresRoot + "/:datastore"
)

//
// Datastore handler.
type DatastoreHandler struct {
	Base
}

//
// Add routes to the `gin` router.
func (h *DatastoreHandler) AddRoutes(e *gin.Engine) {
	e.GET(DatastoresRoot, h.List)
	e.GET(DatastoresRoot+"/", h.List)
	e.GET(DatastoreRoot, h.Get)
}

//
// Prepare to handle the request.
func (h *DatastoreHandler) Prepare(ctx *gin.Context) int {
	return h.Base.Prepare(ctx)
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
	content := []*Datastore{}
	for _, m := range list {
		r := &Datastore{}
		r.With(&m, false)
		content = append(content, r)
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
			ID: ctx.Param("cluster"),
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
	r.With(m, true)

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Datastore struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Capacity    int64        `json:"capacity"`
	Free        int64        `json:"free"`
	Maintenance string       `json:"maintenance"`
	Object      model.Object `json:"object,omitempty"`
}

//
// Build the resource using the model.
func (r *Datastore) With(m *model.Datastore, detail bool) {
	r.ID = m.ID
	r.Name = m.Name
	r.Type = m.Type
	r.Capacity = m.Capacity
	r.Free = m.Free
	r.Maintenance = m.Maintenance
	if detail {
		r.Object = m.DecodeObject()
	}
}
