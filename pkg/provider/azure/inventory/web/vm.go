package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

const (
	VMParam = "vm"
	VMsRoot = ProviderRoot + "/vms"
	VMRoot  = VMsRoot + "/:" + VMParam
)

type VMHandler struct {
	Handler
}

func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

func (h *VMHandler) List(ctx *gin.Context) {
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
	var list []model.VM
	err = db.List(&list, listOptions)
	if err != nil {
		log.Error(err, "Failed to list VMs")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	var result []interface{}
	for _, vm := range list {
		r := &VM{}
		r.WithModel(&vm)
		r.Link(h.Provider)
		if details, err := vm.GetDetails(); err == nil {
			r.Object = details
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

func (h *VMHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	vm := &model.VM{}
	vm.UID = decodeParam(ctx, VMParam)
	log.V(2).Info("VM Get request", "rawParam", ctx.Param(VMParam), "decodedUID", vm.UID)

	db := h.Collector.DB()
	err = db.Get(vm)
	if err != nil {
		log.V(1).Info("VM not found in DB", "uid", vm.UID)
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &VM{}
	r.WithModel(vm)
	r.Link(h.Provider)
	details, err := vm.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

func (h *VMHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.VM{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.VM)
			vm := &VM{}
			vm.WithModel(m)
			vm.Link(h.Provider)
			if details, err := m.GetDetails(); err == nil {
				vm.Object = details
			}
			r = vm
			return
		})
	if err != nil {
		log.Error(err, "watch failed")
		ctx.Status(http.StatusInternalServerError)
	}
}
