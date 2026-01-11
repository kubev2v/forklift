package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
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
// Supports filtering by:
//   - name: Filter by network name (e.g., ?name=my-vpc)
//   - label.*: Filter by AWS tags (e.g., ?label.env=production&label.team=platform)
//
// WebSocket watch supported via X-Watch header.
func (h *NetworkHandler) List(ctx *gin.Context) {
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

	// Build list options with label filtering
	listOptions := h.ListOptionsWithLabels(ctx)

	db := h.Collector.DB()
	var list []model.Network
	err = db.List(&list, listOptions)
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
		if details, err := network.GetDetails(); err == nil {
			r.Object = details
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
	details, err := network.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

// Watch networks via WebSocket.
// Clients can connect with the X-Watch header to receive real-time updates
// when networks are created, updated, or deleted in the inventory.
func (h *NetworkHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Network{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Network)
			network := &Network{}
			network.ID = m.UID
			network.Name = m.Name
			network.Revision = m.Revision
			network.Link(h.Provider)
			if details, err := m.GetDetails(); err == nil {
				network.Object = details
			}
			r = network
			return
		})
	if err != nil {
		log.Error(err, "watch failed")
		ctx.Status(http.StatusInternalServerError)
	}
}
