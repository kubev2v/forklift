package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/provider/ec2/inventory/model"
)

// Routes
const (
	WorkloadsRoot = ProviderRoot + "/workloads"
	WorkloadRoot  = WorkloadsRoot + "/:" + VMParam
)

// Workload handler.
// EC2 workloads are instances with expanded context for migration planning.
type WorkloadHandler struct {
	Handler
}

// Add routes
func (h *WorkloadHandler) AddRoutes(e *gin.Engine) {
	e.GET(WorkloadRoot, h.Get)
}

// Get workload
func (h *WorkloadHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}

	instance := &model.Instance{}
	instance.ID = ctx.Param(VMParam)

	db := h.Collector.DB()
	err = db.Get(instance)
	if err != nil {
		ctx.Status(http.StatusNotFound)
		return
	}

	r := &Workload{}
	r.ID = instance.ID
	r.Name = instance.Name
	r.Revision = instance.Revision
	r.PowerState = instance.PowerState
	r.Link(h.Provider)
	details, err := instance.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}
