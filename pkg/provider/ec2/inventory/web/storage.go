package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Storage handler
type StorageHandler struct {
	Handler
}

// Add routes
func (h *StorageHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot+"/storages", h.List)
	e.GET(ProviderRoot+"/storages/:id", h.Get)
}

// List storage types
func (h *StorageHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	db := h.Collector.DB()
	var list []model.Storage
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Error(err, "Failed to list storage types")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	var result []interface{}
	for _, storage := range list {
		r := &Storage{}
		r.ID = storage.UID
		r.Name = storage.Name
		r.Revision = storage.Revision
		r.Link(h.Provider)
		// Include full object data
		if obj, err := storage.GetObject(); err == nil {
			r.Object = obj.Object
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

// Get storage type
func (h *StorageHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	storage := &model.Storage{}
	storage.UID = ctx.Param("id")

	db := h.Collector.DB()
	err = db.Get(storage)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &Storage{}
	r.ID = storage.UID
	r.Name = storage.Name
	r.Revision = storage.Revision
	r.Link(h.Provider)
	// Include full object data
	obj, err := storage.GetObject()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = obj.Object

	ctx.JSON(http.StatusOK, r)
}
