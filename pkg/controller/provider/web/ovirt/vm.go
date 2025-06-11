package ovirt

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
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

type CpuPinningPolicy string

// CPU Pinning Policies
const (
	None           CpuPinningPolicy = "none"
	Manual         CpuPinningPolicy = "manual"
	ResizeAndPin   CpuPinningPolicy = "resize_and_pin_numa"
	Dedicated      CpuPinningPolicy = "dedicated"
	IsolateThreads CpuPinningPolicy = "isolate_threads"
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
	h.Detail = model.MaxDetail
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
	content := r.Content(h.Detail)

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
		if h.PathMatchRoot(path, name) {
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
	Cluster           string           `json:"cluster"`
	Status            string           `json:"status"`
	Host              string           `json:"host"`
	RevisionValidated int64            `json:"revisionValidated"`
	NICs              []VNIC           `json:"nics"`
	DiskAttachments   []DiskAttachment `json:"diskAttachments"`
	Concerns          []Concern        `json:"concerns"`
}

// Build the resource using the model.
func (r *VM1) With(m *model.VM) {
	r.VM0.With(&m.Base)
	r.Cluster = m.Cluster
	r.Status = m.Status
	r.Host = m.Host
	r.NICs = m.NICs
	r.DiskAttachments = m.DiskAttachments
	r.RevisionValidated = m.RevisionValidated
	r.Concerns = m.Concerns
}

// As content.
func (r *VM1) Content(detail int) interface{} {
	if detail < 1 {
		return &r.VM0
	}

	return r
}

// VM resource.
type VM struct {
	VM1
	PolicyVersion               int              `json:"policyVersion"`
	GuestName                   string           `json:"guestName"`
	CpuSockets                  int16            `json:"cpuSockets"`
	CpuCores                    int16            `json:"cpuCores"`
	CpuThreads                  int16            `json:"cpuThreads"`
	CpuShares                   int16            `json:"cpuShares"`
	CpuAffinity                 []CpuPinning     `json:"cpuAffinity"`
	CpuPinningPolicy            CpuPinningPolicy `json:"cpuPinningPolicy"`
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
	Stateless                   string           `json:"stateless"`
	SerialNumber                string           `json:"serialNumber"`
	HostDevices                 []HostDevice     `json:"hostDevices"`
	CDROMs                      []CDROM          `json:"cdroms"`
	WatchDogs                   []WatchDog       `json:"watchDogs"`
	Properties                  []Property       `json:"properties"`
	Snapshots                   []Snapshot       `json:"snapshots"`
	Guest                       Guest            `json:"guest"`
	OSType                      string           `json:"osType"`
	CustomCpuModel              string           `json:"customCpuModel"`
}

type VNIC = model.NIC
type DiskAttachment = model.DiskAttachment
type IpAddress = model.IpAddress
type CpuPinning = model.CpuPinning
type HostDevice = model.HostDevice
type CDROM = model.CDROM
type WatchDog = model.WatchDog
type Property = model.Property
type Snapshot = model.Snapshot
type Concern = model.Concern
type Guest = model.Guest

// Build the resource using the model.
func (r *VM) With(m *model.VM) {
	r.VM1.With(m)
	r.PolicyVersion = m.PolicyVersion
	r.GuestName = m.GuestName
	r.CpuSockets = m.CpuSockets
	r.CpuCores = m.CpuCores
	r.CpuThreads = m.CpuThreads
	r.CpuShares = m.CpuShares
	r.CpuAffinity = m.CpuAffinity
	if r.CpuPinningPolicy = CpuPinningPolicy(m.CpuPinningPolicy); len(r.CpuPinningPolicy) == 0 {
		r.CpuPinningPolicy = None
	}
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
	r.Stateless = m.Stateless
	r.SerialNumber = m.SerialNumber
	r.HostDevices = m.HostDevices
	r.CDROMs = m.CDROMs
	r.WatchDogs = m.WatchDogs
	r.Properties = m.Properties
	r.Snapshots = m.Snapshots
	r.Guest = m.Guest
	r.OSType = m.OSType
	r.CustomCpuModel = m.CustomCpuModel
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
