package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Network handler
type NetworkHandler struct {
	Handler
}

// Add routes
func (h *NetworkHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot+"/networks", h.List)
	e.GET(ProviderRoot+"/networks/:id", h.Get)
}

// List networks
func (h *NetworkHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	db := h.Collector.DB()
	var list []model.Network
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Error(err, "Failed to list networks")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	var result []interface{}
	for _, network := range list {
		r := &Network{}
		r.ID = network.UID
		r.Name = network.Name
		r.Revision = network.Revision
		r.Link(h.Provider)
		// Include full object data
		if obj, err := network.GetObject(); err == nil {
			r.Object = obj.Object
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

// Get network
func (h *NetworkHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	network := &model.Network{}
	network.UID = ctx.Param("id")

	db := h.Collector.DB()
	err = db.Get(network)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &Network{}
	r.ID = network.UID
	r.Name = network.Name
	r.Revision = network.Revision
	r.Link(h.Provider)
	// Include full object data
	obj, err := network.GetObject()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = obj.Object

	ctx.JSON(http.StatusOK, r)
}
