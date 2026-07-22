package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

type NetworkHandler struct {
	Handler
}

func (h *NetworkHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProviderRoot+"/networks", h.List)
	e.GET(ProviderRoot+"/networks/:id", h.Get)
}

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

	listOptions := h.ListOptions(ctx)

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
		r.WithModel(&network)
		r.Link(h.Provider)
		if details, err := network.GetDetails(); err == nil {
			r.Object = details
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

func (h *NetworkHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	network := &model.Network{}
	network.UID = decodeParam(ctx, "id")

	db := h.Collector.DB()
	err = db.Get(network)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &Network{}
	r.WithModel(network)
	r.Link(h.Provider)
	details, err := network.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

func (h *NetworkHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Network{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Network)
			network := &Network{}
			network.WithModel(m)
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
