package hyperv

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	VMParam      = "vm"
	VMCollection = "vms"
	VMsRoot      = ProviderRoot + "/" + VMCollection
	VMRoot       = VMsRoot + "/:" + VMParam
)

// Virtual Machine handler.
type VMHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

// List resources in a REST collection.
func (h VMHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	db := h.Collector.DB()
	list := []model.VM{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	err = h.filter(ctx, &list)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &VM{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h VMHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	vmID := ctx.Param(VMParam)
	if vmID == "" {
		ctx.Status(http.StatusBadRequest)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: vmID,
		},
	}
	db := h.Collector.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Error(err, "failed to get VM", "vmID", vmID, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &VM{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *VMHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.VM{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.VM)
			vm := &VM{}
			vm.With(m)
			vm.Link(h.Provider)
			r = vm
			return
		})
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

// Filter result set.
func (h *VMHandler) filter(ctx *gin.Context, list *[]model.VM) (err error) {
	if len(*list) < 2 {
		return
	}
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) == 0 {
		return
	}
	if len(strings.Split(name, "/")) < 2 {
		return
	}
	kept := []model.VM{}
	for _, m := range *list {
		if h.PathMatch(m.Name, name) {
			kept = append(kept, m)
		}
	}

	*list = kept
	return
}

// VM detail=0
type VM0 = Resource

// VM detail=1
type VM1 struct {
	VM0
	RevisionValidated int64           `json:"revisionValidated"`
	Disks             []model.Disk    `json:"disks"`
	NICs              []model.NIC     `json:"nics"`
	Networks          []model.Ref     `json:"networks"`
	Concerns          []model.Concern `json:"concerns"`
}

// Build the resource using the model.
func (r *VM1) With(m *model.VM) {
	r.VM0.With(&m.Base)
	r.RevisionValidated = m.RevisionValidated
	r.Disks = m.Disks
	r.NICs = m.NICs
	r.Networks = m.Networks
	r.Concerns = m.Concerns
}

// As content.
func (r *VM1) Content(detail int) interface{} {
	if detail < 1 {
		return &r.VM0
	}
	return r
}

// VM full detail.
type VM struct {
	VM1
	UUID          string               `json:"uuid"`
	PowerState    string               `json:"powerState"`
	CpuCount      int32                `json:"cpuCount"`
	MemoryMB      int32                `json:"memoryMB"`
	Firmware      string               `json:"firmware"`
	GuestOS       string               `json:"guestOS,omitempty"`
	TpmEnabled    bool                 `json:"tpmEnabled"`
	SecureBoot    bool                 `json:"secureBoot"`
	HasCheckpoint bool                 `json:"hasCheckpoint"`
	GuestNetworks []model.GuestNetwork `json:"guestNetworks"`
}

// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.VM1.With(m)
	r.UUID = m.UUID
	r.PowerState = m.PowerState
	r.CpuCount = m.CpuCount
	r.MemoryMB = m.MemoryMB
	r.Firmware = m.Firmware
	r.GuestOS = m.GuestOS
	r.TpmEnabled = m.TpmEnabled
	r.SecureBoot = m.SecureBoot
	r.HasCheckpoint = m.HasCheckpoint
	if m.GuestNetworks != nil {
		r.GuestNetworks = m.GuestNetworks
	} else {
		r.GuestNetworks = []model.GuestNetwork{}
	}
}

// Build self link (URI).
func (r *VM) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
}

// As content.
func (r *VM) Content(detail int) interface{} {
	if detail < 2 {
		return r.VM1.Content(detail)
	}
	return r
}
