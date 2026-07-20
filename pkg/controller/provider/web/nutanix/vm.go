package nutanix

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
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

// VM handler.
type VMHandler struct {
	Handler
}

// Add routes.
func (h *VMHandler) AddRoutes(e *gin.Engine) {
	e.GET(VMsRoot, h.List)
	e.GET(VMsRoot+"/", h.List)
	e.GET(VMRoot, h.Get)
}

// List resources.
func (h VMHandler) List(ctx *gin.Context) {
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
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	db := h.Collector.DB()
	list := []model.VM{}
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		return
	}
	content := []interface{}{}
	err = h.filter(ctx, &list)
	if err != nil {
		return
	}
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &VM{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h VMHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.VM{
		Base: model.Base{
			ID: ctx.Param(VMParam),
		},
	}
	db := h.Collector.DB()
	err = db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	pb := PathBuilder{DB: db}
	r := &VM{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
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
			pb := PathBuilder{DB: db}
			m := in.(*model.VM)
			vm := &VM{}
			vm.With(m)
			vm.Link(h.Provider)
			vm.Path = pb.Path(m)
			r = vm
			return
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
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
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	kept := []model.VM{}
	for _, m := range *list {
		path := pb.Path(&m)
		if path == name || strings.HasSuffix(path, "/"+name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// Base VM resource with minimal detail.
type VM0 struct {
	Resource
	UUID        string `json:"uuid"`
	Cluster     string `json:"cluster"`
	Host        string `json:"host"`
	PowerState  string `json:"powerState"`
	Description string `json:"description,omitempty"`
}

// Build the resource using the model.
func (r *VM0) With(m *model.VM) {
	r.Resource.With(&m.Base)
	r.UUID = m.UUID
	r.Cluster = m.Cluster
	r.Host = m.Host
	r.PowerState = m.PowerState
	r.Description = m.Description
}

// Extended VM resource with medium detail level.
type VM1 struct {
	VM0
	NumSockets        int               `json:"numSockets"`
	NumVcpusPerSocket int               `json:"numVcpusPerSocket"`
	NumThreadsPerCore int               `json:"numThreadsPerCore"`
	MemorySizeMiB     int64             `json:"memorySizeMib"`
	BootType          string            `json:"bootType"`
	BootDeviceOrder   string            `json:"bootDeviceOrder"`
	MachineType       string            `json:"machineType"`
	HardwareClockTZ   string            `json:"hardwareClockTimezone"`
	VGAConsoleEnabled bool              `json:"vgaConsoleEnabled"`
	HypervisorType    string            `json:"hypervisorType"`
	GuestOSID         string            `json:"guestOsId"`
	NICs              []NIC             `json:"nics"`
	Disks             []Disk            `json:"disks"`
	SerialPorts       []SerialPort      `json:"serialPorts"`
	Categories        map[string]string `json:"categories,omitempty"`
}

type NIC = model.NIC
type Disk = model.Disk
type SerialPort = model.SerialPort

// Build the resource using the model.
func (r *VM1) With(m *model.VM) {
	r.VM0.With(m)
	r.NumSockets = m.NumSockets
	r.NumVcpusPerSocket = m.NumVcpusPerSocket
	r.NumThreadsPerCore = m.NumThreadsPerCore
	r.MemorySizeMiB = m.MemorySizeMiB
	r.BootType = m.BootType
	r.BootDeviceOrder = m.BootDeviceOrder
	r.MachineType = m.MachineType
	r.HardwareClockTZ = m.HardwareClockTZ
	r.VGAConsoleEnabled = m.VGAConsoleEnabled
	r.HypervisorType = m.HypervisorType
	r.GuestOSID = m.GuestOSID
	r.NICs = m.NICs
	r.Disks = m.Disks
	r.SerialPorts = m.SerialPorts
	r.Categories = m.Categories
}

// As content.
func (r *VM1) Content(detail int) interface{} {
	if detail < 1 {
		return &r.VM0
	}
	return r
}

// Full VM resource with all details.
type VM struct {
	VM1
	GuestToolsEnabled   bool   `json:"guestToolsEnabled"`
	GuestToolsVersion   string `json:"guestToolsVersion"`
	GuestToolsReachable bool   `json:"guestToolsReachable"`
	GuestToolsMounted   bool   `json:"guestToolsMounted"`
}

// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.VM1.With(m)
	r.GuestToolsEnabled = m.GuestToolsEnabled
	r.GuestToolsVersion = m.GuestToolsVersion
	r.GuestToolsReachable = m.GuestToolsReachable
	r.GuestToolsMounted = m.GuestToolsMounted
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
