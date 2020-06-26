package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"net/http"
)

const (
	NetworksRoot = Root + "/networks"
	NetworkRoot  = NetworksRoot + "/:network"
)

//
// Network handler.
type NetworkHandler struct {
	Base
}

//
// Add routes to the `gin` router.
func (h *NetworkHandler) AddRoutes(e *gin.Engine) {
	e.GET(NetworksRoot, h.List)
	e.GET(NetworksRoot+"/", h.List)
	e.GET(NetworkRoot, h.Get)
}

//
// Prepare to handle the request.
func (h *NetworkHandler) Prepare(ctx *gin.Context) int {
	return h.Base.Prepare(ctx)
}

//
// List resources in a REST collection.
func (h NetworkHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	selector := &model.Network{}
	options := libmodel.ListOptions{
		Page: &h.Page,
	}
	list := []model.Network{}
	err := db.List(selector, options, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []*Network{}
	for _, m := range list {
		r := &Network{}
		r.With(&m, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h NetworkHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Network{
		Base: model.Base{
			ID: ctx.Param("network"),
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
	r := &Network{}
	r.With(m, true)

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Network struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Tag    string       `json:"tag"`
	Object model.Object `json:"object,omitempty"`
}

//
// Build the resource using the model.
func (r *Network) With(m *model.Network, detail bool) {
	r.ID = m.ID
	r.Name = m.Name
	r.Tag = m.Tag
	if detail {
		r.Object = m.DecodeObject()
	}
}
