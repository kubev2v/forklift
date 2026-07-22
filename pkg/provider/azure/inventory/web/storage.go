package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

type StorageHandler struct {
	Handler
}

func (h *StorageHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot+"/storages", h.List)
	e.GET(ProviderRoot+"/storages/:id", h.Get)
}

func (h *StorageHandler) List(ctx *gin.Context) {
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
		if details, err := storage.GetDetails(); err == nil {
			r.Object = details
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

func (h *StorageHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	storage := &model.Storage{}
	storage.UID = decodeParam(ctx, "id")

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
	details, err := storage.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

func (h *StorageHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Storage{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Storage)
			storage := &Storage{}
			storage.ID = m.UID
			storage.Name = m.Name
			storage.Revision = m.Revision
			storage.Link(h.Provider)
			if details, err := m.GetDetails(); err == nil {
				storage.Object = details
			}
			r = storage
			return
		})
	if err != nil {
		log.Error(err, "watch failed")
		ctx.Status(http.StatusInternalServerError)
	}
}
