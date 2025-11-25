package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
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
func (h *VolumeHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	db := h.Collector.DB()
	var list []model.Volume
	err = db.List(&list, h.ListOptions(ctx))
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
		if obj, err := volume.GetObject(); err == nil {
			r.Object = obj.Object
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
	obj, err := volume.GetObject()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = obj.Object

	ctx.JSON(http.StatusOK, r)
}
