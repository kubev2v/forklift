package ova

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ova"
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
	Networks          []model.Network `json:"networks"`
	Disks             []model.Disk    `json:"disks"`
	Concerns          []model.Concern `json:"concerns"`
}

// Build the resource using the model.
func (r *VM1) With(m *model.VM) {
	r.VM0.With(&m.Base)
	r.RevisionValidated = m.RevisionValidated
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
	OvaPath               string          `json:"ovaPath"`
	OvaSource             string          `json:"ovaSource"`
	OsType                string          `json:"osType"`
	RevisionValidated     int64           `json:"revisionValidated"`
	PolicyVersion         int             `json:"policyVersion"`
	UUID                  string          `json:"uuid"`
	Firmware              string          `json:"firmware"`
	SecureBoot            bool            `json:"secureBoot"`
	CpuAffinity           []int32         `json:"cpuAffinity"`
	CpuHotAddEnabled      bool            `json:"cpuHotAddEnabled"`
	CpuHotRemoveEnabled   bool            `json:"cpuHotRemoveEnabled"`
	MemoryHotAddEnabled   bool            `json:"memoryHotAddEnabled"`
	FaultToleranceEnabled bool            `json:"faultToleranceEnabled"`
	CpuCount              int32           `json:"cpuCount"`
	CoresPerSocket        int32           `json:"coresPerSocket"`
	MemoryMB              int32           `json:"memoryMB"`
	MemoryUnits           string          `json:"memoryUnits"`
	CpuUnits              string          `json:"cpuUnits"`
	BalloonedMemory       int32           `json:"balloonedMemory"`
	IpAddress             string          `json:"ipAddress"`
	NumaNodeAffinity      []string        `json:"numaNodeAffinity"`
	StorageUsed           int64           `json:"storageUsed"`
	ChangeTrackingEnabled bool            `json:"changeTrackingEnabled"`
	Devices               []model.Device  `json:"devices"`
	NICs                  []model.NIC     `json:"nics"`
	Disks                 []model.Disk    `json:"disks"`
	Networks              []model.Network `json:"networks"`
}

// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.VM1.With(m)
	r.PolicyVersion = m.PolicyVersion
	r.UUID = m.UUID
	r.Firmware = m.Firmware
	r.SecureBoot = m.SecureBoot
	r.ChangeTrackingEnabled = m.ChangeTrackingEnabled
	r.CpuAffinity = m.CpuAffinity
	r.CpuHotAddEnabled = m.CpuHotAddEnabled
	r.CpuHotRemoveEnabled = m.CpuHotRemoveEnabled
	r.MemoryHotAddEnabled = m.MemoryHotAddEnabled
	r.CpuCount = m.CpuCount
	r.CoresPerSocket = m.CoresPerSocket
	r.MemoryMB = m.MemoryMB
	r.BalloonedMemory = m.BalloonedMemory
	r.MemoryUnits = m.MemoryUnits
	r.CpuUnits = m.CpuUnits
	r.IpAddress = m.IpAddress
	r.StorageUsed = m.StorageUsed
	r.FaultToleranceEnabled = m.FaultToleranceEnabled
	r.Devices = m.Devices
	r.NumaNodeAffinity = m.NumaNodeAffinity
	r.NICs = m.NICs
	r.OvaPath = m.OvaPath
	r.OvaSource = m.OvaSource
	r.OsType = m.OsType
	r.Disks = m.Disks
	r.Networks = m.Networks
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
