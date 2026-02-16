package hyperv

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	WorkloadCollection = "workloads"
	WorkloadsRoot      = Root + "/:" + base.ProviderParam + "/" + WorkloadCollection
	WorkloadRoot       = WorkloadsRoot + "/:" + VMParam
)

// Virtual Machine handler.
type WorkloadHandler struct {
	Handler
	Config Config
}

// Add routes to the `gin` router.
func (h *WorkloadHandler) AddRoutes(e *gin.Engine) {
	e.GET(WorkloadRoot, h.Get)
}

// List resources in a REST collection.
func (h WorkloadHandler) List(ctx *gin.Context) {
}

// Get a specific REST resource.
func (h WorkloadHandler) Get(ctx *gin.Context) {
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
	db := h.Collector.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	if err != nil {
		return
	}
	r := Workload{}
	r.With(m)
	r.config = h.Config
	err = r.Expand(db)
	if err != nil {
		return
	}
	r.Link(h.Provider)
	content := r

	ctx.JSON(http.StatusOK, content)
}

type Workload struct {
	SelfLink string `json:"selfLink"`
	// Embed VM fields for validation
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	UUID          string               `json:"uuid"`
	Firmware      string               `json:"firmware"`
	CpuCount      int32                `json:"cpuCount"`
	MemoryMB      int32                `json:"memoryMB"`
	PowerState    string               `json:"powerState"`
	GuestOS       string               `json:"guestOS,omitempty"`
	TpmEnabled    bool                 `json:"tpmEnabled"`
	SecureBoot    bool                 `json:"secureBoot"`
	HasCheckpoint bool                 `json:"hasCheckpoint"`
	Disks         []model.Disk         `json:"disks"`
	NICs          []model.NIC          `json:"nics"`
	GuestNetworks []model.GuestNetwork `json:"guestNetworks"`
	Concerns      []model.Concern      `json:"concerns"`
	config        Config               // unexported, not serialized
}

// With populates the workload from a VM model.
func (r *Workload) With(m *model.VM) {
	r.ID = m.ID
	r.Name = m.Name
	r.UUID = m.UUID
	r.Firmware = m.Firmware
	r.CpuCount = m.CpuCount
	r.MemoryMB = m.MemoryMB
	r.PowerState = m.PowerState
	r.GuestOS = m.GuestOS
	r.TpmEnabled = m.TpmEnabled
	r.SecureBoot = m.SecureBoot
	r.HasCheckpoint = m.HasCheckpoint
	r.Disks = m.Disks
	r.NICs = m.NICs
	r.GuestNetworks = m.GuestNetworks
	r.Concerns = m.Concerns
	// Ensure non-nil slices for JSON serialization
	if r.Disks == nil {
		r.Disks = []model.Disk{}
	}
	if r.NICs == nil {
		r.NICs = []model.NIC{}
	}
	if r.GuestNetworks == nil {
		r.GuestNetworks = []model.GuestNetwork{}
	}
	if r.Concerns == nil {
		r.Concerns = []model.Concern{}
	}
}

// Build self link (URI).
func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		WorkloadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}

// Expand the resource.
func (r *Workload) Expand(db libmodel.DB) (err error) {
	vm := &model.VM{
		Base: model.Base{ID: r.ID},
	}
	err = db.Get(vm)
	return
}
