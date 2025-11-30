package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Routes
const (
	VMParam = "vm"
	VMsRoot = ProviderRoot + "/vms"
	VMRoot  = VMsRoot + "/:" + VMParam
)

// VM handler
type VMHandler struct {
	Handler
}

// Add routes
func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

// List VMs
func (h *VMHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	db := h.Collector.DB()
	var list []model.Instance
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Error(err, "Failed to list instances")
		ctx.Status(http.StatusInternalServerError)
		return
	}

	// Convert to VM resources
	var result []interface{}
	for _, instance := range list {
		r := &VM{}
		r.ID = instance.UID
		r.Name = instance.Name
		r.Revision = instance.Revision
		r.Link(h.Provider)
		// Include full object data
		if obj, err := instance.GetObject(); err == nil {
			r.Object = obj.Object
		}
		result = append(result, r)
	}

	ctx.JSON(http.StatusOK, result)
}

// Get VM
func (h *VMHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	instance := &model.Instance{}
	instance.UID = ctx.Param(VMParam)

	db := h.Collector.DB()
	err = db.Get(instance)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &VM{}
	r.ID = instance.UID
	r.Name = instance.Name
	r.Revision = instance.Revision
	r.Link(h.Provider)
	// Include full object data
	obj, err := instance.GetObject()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = obj.Object

	ctx.JSON(http.StatusOK, r)
}
