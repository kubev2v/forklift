package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
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
// Supports filtering by:
//   - name: Filter by instance name (e.g., ?name=my-instance)
//   - label.*: Filter by AWS tags (e.g., ?label.env=production&label.team=platform)
//
// WebSocket watch supported via X-Watch header.
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

	// Build list options with label filtering
	listOptions := h.ListOptionsWithLabels(ctx)

	db := h.Collector.DB()
	var list []model.Instance
	err = db.List(&list, listOptions)
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
		if details, err := instance.GetDetails(); err == nil {
			r.Object = details
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
	details, err := instance.GetDetails()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Object = details

	ctx.JSON(http.StatusOK, r)
}

// Watch VMs via WebSocket.
// Clients can connect with the X-Watch header to receive real-time updates
// when instances are created, updated, or deleted in the inventory.
func (h *VMHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Instance{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Instance)
			vm := &VM{}
			vm.ID = m.UID
			vm.Name = m.Name
			vm.Revision = m.Revision
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
