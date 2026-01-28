package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Volume (disk) handler
type VolumeHandler struct {
	Handler
}

// Add routes
func (h *VolumeHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot+"/volumes", h.List)
	e.GET(ProviderRoot+"/volumes/:id", h.Get)
}

// List volumes
// Supports filtering by:
//   - name: Filter by volume name (e.g., ?name=my-volume)
//   - label.*: Filter by AWS tags (e.g., ?label.env=production&label.team=platform)
//
// WebSocket watch supported via X-Watch header.
func (h *VolumeHandler) List(ctx *gin.Context) {
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
	var list []model.Volume
	err = db.List(&list, listOptions)
	if err != nil {
		log.Error(err, "Failed to list volumes")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	var result []interface{}
	for _, volume := range list {
		r := &Volume{}
		r.ID = volume.UID
		r.Name = volume.Name
		r.Revision = volume.Revision
		r.Link(h.Provider)
		// Include full object data
		if details, err := volume.GetDetails(); err == nil {
			r.Object = details
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

// Get volume
func (h *VolumeHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	volume := &model.Volume{}
	volume.UID = ctx.Param("id")

	db := h.Collector.DB()
	err = db.Get(volume)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &Volume{}
	r.ID = volume.UID
	r.Name = volume.Name
	r.Revision = volume.Revision
	r.Link(h.Provider)
	// Include full object data
	details, err := volume.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

// Watch volumes via WebSocket.
// Clients can connect with the X-Watch header to receive real-time updates
// when volumes are created, updated, or deleted in the inventory.
func (h *VolumeHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Volume{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Volume)
			volume := &Volume{}
			volume.ID = m.UID
			volume.Name = m.Name
			volume.Revision = m.Revision
			volume.Link(h.Provider)
			if details, err := m.GetDetails(); err == nil {
				volume.Object = details
			}
			r = volume
			return
		})
	if err != nil {
		log.Error(err, "watch failed")
		ctx.Status(http.StatusInternalServerError)
	}
}
