package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
	"strings"
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
	Handler
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
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
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
	db := h.Reconciler.DB()
	list := []model.VM{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	err = h.filter(ctx, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
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
	r.Path, err = m.Path(db)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
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
			base.ProviderParam: string(p.UID),
			VMParam:            m.ID,
		})
}

//
// Watch.
func (h VMHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.VM{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.VM)
			vm := &VM{}
			vm.With(m)
			vm.SelfLink = h.Link(h.Provider, m)
			vm.Path, _ = m.Path(db)
			r = vm
			return
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
	}
}

//
// Filter result set.
// Filter by path for `name` query.
func (h VMHandler) filter(ctx *gin.Context, list *[]model.VM) (err error) {
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
	db := h.Reconciler.DB()
	kept := []model.VM{}
	for _, m := range *list {
		path, pErr := m.Path(db)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		if h.PathMatch(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

//
// REST Resource.
type VM struct {
	Resource
	PolicyVersion         int             `json:"policyVersion"`
	RevisionValidated     int64           `json:"revisionValidated"`
	UUID                  string          `json:"uuid"`
	Firmware              string          `json:"firmware"`
	PowerState            string          `json:"powerState"`
	Snapshot              model.Ref       `json:"snapshot"`
	ChangeTrackingEnabled bool            `json:"changeTrackingEnabled"`
	CpuAffinity           []int32         `json:"cpuAffinity"`
	CpuHotAddEnabled      bool            `json:"cpuHotAddEnabled"`
	CpuHotRemoveEnabled   bool            `json:"cpuHotRemoveEnabled"`
	MemoryHotAddEnabled   bool            `json:"memoryHotAddEnabled"`
	FaultToleranceEnabled bool            `json:"faultToleranceEnabled"`
	CpuCount              int32           `json:"cpuCount"`
	CoresPerSocket        int32           `json:"coresPerSocket"`
	MemoryMB              int32           `json:"memoryMB"`
	GuestName             string          `json:"guestName"`
	BalloonedMemory       int32           `json:"balloonedMemory"`
	IpAddress             string          `json:"ipAddress"`
	StorageUsed           int64           `json:"storageUsed"`
	NumaNodeAffinity      []string        `json:"numaNodeAffinity"`
	Devices               []model.Device  `json:"devices"`
	Networks              []model.Ref     `json:"networks"`
	Disks                 []model.Disk    `json:"disks"`
	Host                  model.Ref       `json:"host"`
	Concerns              []model.Concern `json:"concerns"`
}

//
// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.Resource.With(&m.Base)
	r.PolicyVersion = m.PolicyVersion
	r.RevisionValidated = m.RevisionValidated
	r.UUID = m.UUID
	r.Firmware = m.Firmware
	r.PowerState = m.PowerState
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
	r.BalloonedMemory = m.BalloonedMemory
	r.IpAddress = m.IpAddress
	r.StorageUsed = m.StorageUsed
	r.FaultToleranceEnabled = m.FaultToleranceEnabled
	r.Devices = m.Devices
	r.NumaNodeAffinity = m.NumaNodeAffinity
	r.Networks = m.Networks
	r.Disks = m.Disks
	r.Host = m.Host
	r.Concerns = m.Concerns
}

//
// As content.
func (r *VM) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
