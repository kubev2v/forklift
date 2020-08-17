package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	VMsRoot = Root + "/vms"
	VMRoot  = VMsRoot + "/:vm"
)

//
// Virtual Machine handler.
type VMHandler struct {
	base.Handler
}

//
// Add routes to the `gin` router.
func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

//
// List resources in a REST collection.
func (h VMHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.VM{}
	err := db.List(
		&list,
		libmodel.ListOptions{
			Page: &h.Page,
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &VM{}
		r.With(&m)
		obj := r.Object(h.Detail)
		content = append(content, obj)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h VMHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: ctx.Param("vm"),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &VM{}
	r.With(m)

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type VM struct {
	base.Resource
	UUID                string `json:"uuid"`
	Firmware            string `json:"firmware"`
	CpuAffinity         string `json:"cpuAffinity"`
	CpuHotAddEnabled    bool   `json:"cpuHostAddEnabled"`
	CpuHotRemoveEnabled bool   `json:"cpuHostRemoveEnabled"`
	MemoryHotAddEnabled bool   `json:"memoryHotAddEnabled"`
	CpuCount            int32  `json:"cpuCount"`
	MemorySizeMB        int32  `json:"memorySizeMB"`
	GuestName           string `json:"guestName"`
	BalloonedMemory     int32  `json:"balloonedMemory"`
	IpAddress           string `json:"ipAddress"`
}

//
// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.Resource.With(&m.Base)
	r.UUID = m.UUID
	r.Firmware = m.Firmware
	r.CpuAffinity = m.CpuAffinity
	r.CpuHotAddEnabled = m.CpuHotAddEnabled
	r.CpuHotRemoveEnabled = m.CpuHotRemoveEnabled
	r.MemoryHotAddEnabled = m.MemoryHotAddEnabled
	r.CpuCount = m.CpuCount
	r.MemorySizeMB = m.MemorySizeMB
	r.GuestName = m.GuestName
	r.BalloonedMemory = m.BalloonedMemory
	r.IpAddress = m.IpAddress
}

//
// Render.
func (r *VM) Object(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
