package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"net/http"
)

const (
	HostsRoot = Root + "/hosts"
	HostRoot  = HostsRoot + "/:host"
)

//
// Host handler.
type HostHandler struct {
	Base
}

//
// Add routes to the `gin` router.
func (h *HostHandler) AddRoutes(e *gin.Engine) {
	e.GET(HostsRoot, h.List)
	e.GET(HostsRoot+"/", h.List)
	e.GET(HostRoot, h.Get)
}

//
// Prepare to handle the request.
func (h *HostHandler) Prepare(ctx *gin.Context) int {
	return h.Base.Prepare(ctx)
}

//
// List resources in a REST collection.
func (h HostHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	selector := &model.Host{}
	options := libmodel.ListOptions{
		Page: &h.Page,
	}
	list := []model.Host{}
	err := db.List(selector, options, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []*Host{}
	for _, m := range list {
		r := &Host{}
		r.With(&m, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h HostHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Host{
		Base: model.Base{
			ID: ctx.Param("host"),
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
	r := &Host{}
	r.With(m, true)

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Host struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Maintenance string       `json:"maintenance"`
	Object      model.Object `json:"object,omitempty"`
}

//
// Build the resource using the model.
func (r *Host) With(m *model.Host, detail bool) {
	r.ID = m.ID
	r.Name = m.Name
	r.Maintenance = m.Maintenance
	if detail {
		r.Object = m.DecodeObject()
	}
}
