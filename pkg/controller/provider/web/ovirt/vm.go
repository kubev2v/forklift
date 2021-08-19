package ovirt

import (
	"errors"
	"github.com/gin-gonic/gin"
	liberr "github.com/konveyor/controller/pkg/error"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
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
	db := h.Collector.DB()
	list := []model.VM{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	err = h.filter(ctx, &list)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, m := range list {
		r := &VM{}
		r.With(&m)
		err = h.Expand(r)
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r.Link(h.Provider)
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
	h.Detail = true
	db := h.Collector.DB()
	err := db.Get(m)
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
	r := &VM{}
	r.With(m)
	err = h.Expand(r)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Link(h.Provider)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Expend the resource.
func (h *VMHandler) Expand(r *VM) (err error) {
	if !h.Detail {
		return
	}
	err = r.Expand(h.Collector.DB())
	return
}

//
// Watch.
func (h VMHandler) watch(ctx *gin.Context) {
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
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

//
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
	kept := []model.VM{}
	for _, m := range *list {
		path, pErr := m.Path(db)
		if pErr != nil {
			err = liberr.Wrap(pErr)
			return
		}
		if h.PathMatchRoot(path, name) {
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
	Cluster                     string           `json:"cluster"`
	Host                        string           `json:"host"`
	RevisionValidated           int64            `json:"revisionValidated"`
	PolicyVersion               int              `json:"policyVersion"`
	GuestName                   string           `json:"guestName"`
	CpuSockets                  int16            `json:"cpuSockets"`
	CpuCores                    int16            `json:"cpuCores"`
	CpuShares                   int16            `json:"cpuShares"`
	CpuAffinity                 []CpuPinning     `json:"cpuAffinity"`
	Memory                      int64            `json:"memory"`
	BalloonedMemory             bool             `json:"balloonedMemory"`
	IOThreads                   int16            `json:"ioThreads"`
	BIOS                        string           `json:"bios"`
	Display                     string           `json:"display"`
	HasIllegalImages            bool             `json:"hasIllegalImages"`
	NumaNodeAffinity            []string         `json:"numaNodeAffinity"`
	LeaseStorageDomain          string           `json:"leaseStorageDomain"`
	StorageErrorResumeBehaviour string           `json:"storageErrorResumeBehaviour"`
	HaEnabled                   bool             `json:"haEnabled"`
	UsbEnabled                  bool             `json:"usbEnabled"`
	BootMenuEnabled             bool             `json:"bootMenuEnabled"`
	PlacementPolicyAffinity     string           `json:"placementPolicyAffinity"`
	Timezone                    string           `json:"timezone"`
	Status                      string           `json:"status"`
	Stateless                   string           `json:"stateless"`
	NICs                        []vNIC           `json:"nics"`
	DiskAttachments             []DiskAttachment `json:"diskAttachments"`
	HostDevices                 []HostDevice     `json:"hostDevices"`
	CDROMs                      []CDROM          `json:"cdroms"`
	WatchDogs                   []WatchDog       `json:"watchDogs"`
	Properties                  []Property       `json:"properties"`
	Snapshots                   []Snapshot       `json:"snapshots"`
	Concerns                    []Concern        `json:"concerns"`
}

type IpAddress = model.IpAddress
type CpuPinning = model.CpuPinning
type HostDevice = model.HostDevice
type CDROM = model.CDROM
type WatchDog = model.WatchDog
type Property = model.Property
type Snapshot = model.Snapshot
type Concern = model.Concern

type vNIC struct {
	model.NIC
	Profile   NICProfile  `json:"profile"`
	Plugged   bool        `json:"plugged"`
	IpAddress []IpAddress `json:"ipAddress"`
}

type DiskAttachment struct {
	model.DiskAttachment
	Disk Disk `json:"disk"`
}

//
// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.Resource.With(&m.Base)
	r.Cluster = m.Cluster
	r.Host = m.Host
	r.RevisionValidated = m.RevisionValidated
	r.PolicyVersion = m.PolicyVersion
	r.GuestName = m.GuestName
	r.CpuSockets = m.CpuSockets
	r.CpuCores = m.CpuCores
	r.CpuShares = m.CpuShares
	r.CpuAffinity = m.CpuAffinity
	r.Memory = m.Memory
	r.BalloonedMemory = m.BalloonedMemory
	r.IOThreads = m.IOThreads
	r.BIOS = m.BIOS
	r.Display = m.Display
	r.HasIllegalImages = m.HasIllegalImages
	r.NumaNodeAffinity = m.NumaNodeAffinity
	r.LeaseStorageDomain = m.LeaseStorageDomain
	r.StorageErrorResumeBehaviour = m.StorageErrorResumeBehaviour
	r.HaEnabled = m.HaEnabled
	r.UsbEnabled = m.UsbEnabled
	r.BootMenuEnabled = m.BootMenuEnabled
	r.PlacementPolicyAffinity = m.PlacementPolicyAffinity
	r.Timezone = m.Timezone
	r.Status = m.Status
	r.Stateless = m.Stateless
	r.HostDevices = m.HostDevices
	r.CDROMs = m.CDROMs
	r.WatchDogs = m.WatchDogs
	r.Properties = m.Properties
	r.Snapshots = m.Snapshots
	r.Concerns = m.Concerns
	r.addDiskAttachment(m)
	r.addNICs(m)
}

//
// Build self link (URI).
func (r *VM) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		VMRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
	for i := range r.NICs {
		n := &r.NICs[i]
		n.Profile.Link(p)
	}
	for i := range r.DiskAttachments {
		d := &r.DiskAttachments[i]
		d.Disk.Link(p)
	}
}

//
// Expand the resource.
// The vNIC profile.ID is optional.
func (r *VM) Expand(db libmodel.DB) (err error) {
	defer func() {
		if err != nil {
			err = liberr.Wrap(err, "vm", r.ID)
		}
	}()
	for i := range r.NICs {
		nic := &r.NICs[i]
		if nic.Profile.ID == "" {
			continue
		}
		profile := &model.NICProfile{
			Base: model.Base{ID: nic.Profile.ID},
		}
		err = db.Get(profile)
		if err != nil {
			return
		}
		nic.Profile.With(profile)
	}
	for i := range r.DiskAttachments {
		d := &r.DiskAttachments[i]
		disk := &model.Disk{
			Base: model.Base{ID: d.Disk.ID},
		}
		err = db.Get(disk)
		if err != nil {
			return
		}
		d.Disk.With(disk)
		err = d.Disk.Expand(db)
		if err != nil {
			return
		}
	}

	return
}

func (r *VM) addDiskAttachment(m *model.VM) {
	r.DiskAttachments = []DiskAttachment{}
	for _, d := range m.DiskAttachments {
		r.DiskAttachments = append(
			r.DiskAttachments,
			DiskAttachment{
				DiskAttachment: d,
				Disk: Disk{
					Resource: Resource{
						ID: d.Disk,
					},
				},
			})
	}
}

func (r *VM) addNICs(m *model.VM) {
	r.NICs = []vNIC{}
	for _, n := range m.NICs {
		r.NICs = append(
			r.NICs,
			vNIC{
				NIC: n,
				Profile: NICProfile{
					Resource: Resource{
						ID: n.Profile,
					},
				},
				Plugged:   n.Plugged,
				IpAddress: n.IpAddress,
			})
	}
}

//
// As content.
func (r *VM) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
