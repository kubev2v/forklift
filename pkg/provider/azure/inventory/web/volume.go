package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

type DiskHandler struct {
	Handler
}

func (h *DiskHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot+"/disks", h.List)
	e.GET(ProviderRoot+"/disks/:id", h.Get)
}

func (h *DiskHandler) List(ctx *gin.Context) {
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

	listOptions := h.ListOptionsWithLabels(ctx)

	db := h.Collector.DB()
	var list []model.Disk
	err = db.List(&list, listOptions)
	if err != nil {
		log.Error(err, "Failed to list disks")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	var result []interface{}
	for _, disk := range list {
		r := &Disk{}
		r.ID = disk.UID
		r.Name = disk.Name
		r.Revision = disk.Revision
		r.Link(h.Provider)
		if details, err := disk.GetDetails(); err == nil {
			r.Object = details
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

func (h *DiskHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	disk := &model.Disk{}
	disk.UID = decodeParam(ctx, "id")

	db := h.Collector.DB()
	err = db.Get(disk)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &Disk{}
	r.ID = disk.UID
	r.Name = disk.Name
	r.Revision = disk.Revision
	r.Link(h.Provider)
	details, err := disk.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

func (h *DiskHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Disk{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Disk)
			disk := &Disk{}
			disk.ID = m.UID
			disk.Name = m.Name
			disk.Revision = m.Revision
			disk.Link(h.Provider)
			if details, err := m.GetDetails(); err == nil {
				disk.Object = details
			}
			r = disk
			return
		})
	if err != nil {
		log.Error(err, "watch failed")
		ctx.Status(http.StatusInternalServerError)
	}
}
