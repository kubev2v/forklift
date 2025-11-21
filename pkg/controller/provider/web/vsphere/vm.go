package vsphere

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
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
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
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

	// We need model.MaxDetail for retrieving IsTemplate field from the database
	h.Detail = model.MaxDetail
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
		if m.IsTemplate {
			log.Info(
				"Skipping template VM",
				"vmID", m.ID,
				"isTemplate", m.IsTemplate)
			continue
		}
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
// Filter by path for `name` query.
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
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	kept := []model.VM{}
	for _, m := range *list {
		path := pb.Path(&m)
		if h.PathMatch(path, name) {
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
	IsTemplate        bool            `json:"isTemplate"`
	PowerState        string          `json:"powerState"`
	Host              string          `json:"host"`
	Networks          []model.Ref     `json:"networks"`
	Disks             []model.Disk    `json:"disks"`
	Concerns          []model.Concern `json:"concerns"`
}

// Build the resource using the model.
func (r *VM1) With(m *model.VM) {
	r.VM0.With(&m.Base)
	r.RevisionValidated = m.RevisionValidated
	r.IsTemplate = m.IsTemplate
	r.PowerState = m.PowerState
	r.Host = m.Host
	r.Networks = m.Networks
	r.Disks = m.Disks
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
	PolicyVersion            int                    `json:"policyVersion"`
	UUID                     string                 `json:"uuid"`
	Firmware                 string                 `json:"firmware"`
	ConnectionState          string                 `json:"connectionState"`
	Snapshot                 model.Ref              `json:"snapshot"`
	ChangeTrackingEnabled    bool                   `json:"changeTrackingEnabled"`
	CpuAffinity              []int32                `json:"cpuAffinity"`
	CpuHotAddEnabled         bool                   `json:"cpuHotAddEnabled"`
	CpuHotRemoveEnabled      bool                   `json:"cpuHotRemoveEnabled"`
	MemoryHotAddEnabled      bool                   `json:"memoryHotAddEnabled"`
	FaultToleranceEnabled    bool                   `json:"faultToleranceEnabled"`
	CpuCount                 int32                  `json:"cpuCount"`
	CoresPerSocket           int32                  `json:"coresPerSocket"`
	MemoryMB                 int32                  `json:"memoryMB"`
	GuestName                string                 `json:"guestName"`
	GuestNameFromVmwareTools string                 `json:"guestNameFromVmwareTools"`
	HostName                 string                 `json:"hostName"`
	GuestID                  string                 `json:"guestId"`
	BalloonedMemory          int32                  `json:"balloonedMemory"`
	IpAddress                string                 `json:"ipAddress"`
	StorageUsed              int64                  `json:"storageUsed"`
	TpmEnabled               bool                   `json:"tpmEnabled"`
	NumaNodeAffinity         []string               `json:"numaNodeAffinity"`
	Devices                  []model.Device         `json:"devices"`
	NICs                     []model.NIC            `json:"nics"`
	GuestNetworks            []model.GuestNetwork   `json:"guestNetworks"`
	GuestDisks               []model.DiskMountPoint `json:"guestDisks"`
	GuestIpStacks            []model.GuestIpStack   `json:"guestIpStacks"`
	SecureBoot               bool                   `json:"secureBoot"`
	ToolsStatus              string                 `json:"toolsStatus"`
	ToolsRunningStatus       string                 `json:"toolsRunningStatus"`
	// Note: vSphere reports version as "toolsVersionStatus2"; we keep the Go field
	// name ToolsVersionStatus for continuity while serializing as toolsVersionStatus2.
	ToolsVersionStatus string `json:"toolsVersionStatus2"`
	DiskEnableUuid     bool   `json:"diskEnableUuid"`
	NestedHVEnabled    bool   `json:"nestedHVEnabled"`
	DRSEnabled         bool   `json:"drsEnabled"`
}

// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.VM1.With(m)
	r.PolicyVersion = m.PolicyVersion
	r.UUID = m.UUID
	r.Firmware = m.Firmware
	r.ConnectionState = m.ConnectionState
	r.Snapshot = m.Snapshot
	r.ChangeTrackingEnabled = m.ChangeTrackingEnabled
	r.CpuAffinity = m.CpuAffinity
	r.CpuHotAddEnabled = m.CpuHotAddEnabled
	r.CpuHotRemoveEnabled = m.CpuHotRemoveEnabled
	r.MemoryHotAddEnabled = m.MemoryHotAddEnabled
	r.CpuCount = m.CpuCount
	r.CoresPerSocket = m.CoresPerSocket
	r.MemoryMB = m.MemoryMB
	r.GuestName = m.GuestName
	r.GuestNameFromVmwareTools = m.GuestNameFromVmwareTools
	r.HostName = m.HostName
	r.GuestID = m.GuestID
	r.BalloonedMemory = m.BalloonedMemory
	r.IpAddress = m.IpAddress
	r.StorageUsed = m.StorageUsed
	r.TpmEnabled = m.TpmEnabled
	r.FaultToleranceEnabled = m.FaultToleranceEnabled
	r.Devices = m.Devices
	r.NumaNodeAffinity = m.NumaNodeAffinity
	r.NICs = m.NICs
	r.GuestNetworks = m.GuestNetworks
	r.GuestDisks = m.GuestDisks
	r.GuestIpStacks = m.GuestIpStacks
	r.SecureBoot = m.SecureBoot
	r.ToolsStatus = m.ToolsStatus
	r.ToolsRunningStatus = m.ToolsRunningStatus
	r.ToolsVersionStatus = m.ToolsVersionStatus
	r.DiskEnableUuid = m.DiskEnableUuid
	r.NestedHVEnabled = m.NestedHVEnabled
	r.DRSEnabled = m.DRSEnabled
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

func (r *VM) HasDisk(disk model.Disk) bool {
	for _, d := range r.Disks {
		if d.File == disk.File {
			return true
		}
	}
	return false
}

func (r *VM) HasSharedDisk() bool {
	for _, d := range r.Disks {
		if d.Shared {
			return true
		}
	}
	return false
}

func (r *VM) RemoveSharedDisks() {
	var disks []model.Disk
	for _, disk := range r.Disks {
		if !disk.Shared {
			disks = append(disks, disk)
		}
	}
	r.Disks = disks
}

func (r *VM) RemoveDisk(removeDisk model.Disk) {
	var disks []model.Disk
	for _, disk := range r.Disks {
		if disk.File != removeDisk.File {
			disks = append(disks, disk)
		}
	}
	r.Disks = disks
}
