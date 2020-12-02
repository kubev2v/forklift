package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	VMParam      = "vm"
	VMCollection = "vms"
	VMsRoot      = ProviderRoot + "/" + VMCollection
	VMRoot       = VMsRoot + "/:" + VMParam
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
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
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
			ID: ctx.Param(VMParam),
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
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h VMHandler) Link(p *api.Provider, m *model.VM) string {
	return h.Handler.Link(
		VMRoot,
		base.Params{
			base.NsParam:       p.Namespace,
			base.ProviderParam: p.Name,
			VMParam:            m.ID,
		})
}

//
// REST Resource.
type VM struct {
	Resource
	UUID                  string        `json:"uuid"`
	Firmware              string        `json:"firmware"`
	CpuAffinity           model.List    `json:"cpuAffinity"`
	CpuHotAddEnabled      bool          `json:"cpuHostAddEnabled"`
	CpuHotRemoveEnabled   bool          `json:"cpuHostRemoveEnabled"`
	MemoryHotAddEnabled   bool          `json:"memoryHotAddEnabled"`
	FaultToleranceEnabled bool          `json:"faultToleranceEnabled"`
	CpuCount              int32         `json:"cpuCount"`
	CoresPerSocket        int32         `json:"coresPerSocket"`
	MemoryMB              int32         `json:"memoryMB"`
	GuestName             string        `json:"guestName"`
	BalloonedMemory       int32         `json:"balloonedMemory"`
	IpAddress             string        `json:"ipAddress"`
	StorageUsed           int64         `json:"storageUsed"`
	NumaNodeAffinity      model.List    `json:"numaNodeAffinity"`
	SriovSupported        bool          `json:"sriovSupported"`
	PassthroughSupported  bool          `json:"passthroughSupported"`
	UsbSupported          bool          `json:"usbSupported"`
	Networks              model.RefList `json:"networks"`
	Disks                 []model.Disk  `json:"disks"`
	Host                  model.Ref     `json:"host"`
	RevisionAnalyzed      int64         `json:"revisionAnalyzed"`
	Concerns              model.List    `json:"concerns"`
}

//
// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.Resource.With(&m.Base)
	r.UUID = m.UUID
	r.Firmware = m.Firmware
	r.CpuAffinity = *(&model.List{}).With(m.CpuAffinity)
	r.CpuHotAddEnabled = m.CpuHotAddEnabled
	r.CpuHotRemoveEnabled = m.CpuHotRemoveEnabled
	r.MemoryHotAddEnabled = m.MemoryHotAddEnabled
	r.CpuCount = m.CpuCount
	r.CoresPerSocket = m.CoresPerSocket
	r.MemoryMB = m.MemoryMB
	r.GuestName = m.GuestName
	r.BalloonedMemory = m.BalloonedMemory
	r.IpAddress = m.IpAddress
	r.StorageUsed = m.StorageUsed
	r.FaultToleranceEnabled = m.FaultToleranceEnabled
	r.SriovSupported = m.SriovSupported
	r.PassthroughSupported = m.PassthroughSupported
	r.UsbSupported = m.UsbSupported
	r.NumaNodeAffinity = *(&model.List{}).With(m.NumaNodeAffinity)
	r.Networks = *model.RefListPtr().With(m.Networks)
	r.Disks = m.DecodeDisks()
	r.Host = *(&model.Ref{}).With(m.Host)
	r.RevisionAnalyzed = m.RevisionAnalyzed
	r.Concerns = *(&model.List{}).With(m.Concerns)
}

//
// As content.
func (r *VM) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
