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
	WorkloadCollection = "workloads"
	WorkloadsRoot      = ProviderRoot + "/" + WorkloadCollection
	WorkloadRoot       = WorkloadsRoot + "/:" + VMParam
)

// Virtual Machine handler.
type WorkloadHandler struct {
	Handler
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
	h.Detail = model.MaxDetail
	r := Workload{}
	r.With(m)
	err = r.Expand(h.Collector.DB())
	if err != nil {
		return
	}
	r.Link(h.Provider)

	ctx.JSON(http.StatusOK, r)
}

// Expanded: Disk.
type XDisk struct {
	Disk
	Profile DiskProfile `json:"profile"`
}

// Expand references.
func (r *XDisk) Expand(db libmodel.DB) (err error) {
	if r.Disk.Profile == "" {
		return
	}
	p := &model.DiskProfile{
		Base: model.Base{ID: r.Disk.Profile},
	}
	err = db.Get(p)
	if err != nil {
		return
	}
	r.Profile.With(p)
	return
}

// Build self link (URI).
func (r *XDisk) Link(p *api.Provider) {
	r.Disk.Link(p)
	r.Profile.Link(p)
}

// Expanded: DiskAttachment.
type XDiskAttachment struct {
	DiskAttachment
	Disk XDisk `json:"disk"`
}

func (r *XDiskAttachment) Expand(db libmodel.DB) (err error) {
	disk := &model.Disk{
		Base: model.Base{ID: r.DiskAttachment.ID},
	}
	disk.ID = r.DiskAttachment.Disk
	err = db.Get(disk)
	if err != nil {
		return
	}
	r.Disk.With(disk)
	err = r.Disk.Expand(db)
	return
}

// Build self link (URI).
func (r *XDiskAttachment) Link(p *api.Provider) {
	r.Disk.Link(p)
}

// Expanded: vNIC.
type XNIC struct {
	VNIC
	Profile NICProfile `json:"profile"`
}

// Expand references.
func (r *XNIC) Expand(db libmodel.DB) (err error) {
	if r.VNIC.Profile == "" {
		return
	}
	p := &model.NICProfile{
		Base: model.Base{ID: r.VNIC.Profile},
	}
	err = db.Get(p)
	if err != nil {
		return
	}
	r.Profile.With(p)
	return
}

// Build self link (URI).
func (r *XNIC) Link(p *api.Provider) {
	r.Profile.Link(p)
}

// Expanded: VM.
type XVM struct {
	VM
	DiskAttachments []XDiskAttachment `json:"diskAttachments"`
	NICs            []XNIC            `json:"nics"`
}

// Expand references.
func (r *XVM) Expand(db libmodel.DB) (err error) {
	for _, real := range r.VM.DiskAttachments {
		expanded := XDiskAttachment{DiskAttachment: real}
		err = expanded.Expand(db)
		if err != nil {
			return
		}
		r.DiskAttachments = append(
			r.DiskAttachments,
			expanded)
	}
	for _, real := range r.VM.NICs {
		expanded := XNIC{VNIC: real}
		err = expanded.Expand(db)
		if err != nil {
			return
		}
		r.NICs = append(
			r.NICs,
			expanded)
	}

	return
}

// Build self link (URI).
func (r *XVM) Link(p *api.Provider) {
	for i := range r.DiskAttachments {
		da := &r.DiskAttachments[i]
		da.Link(p)
	}
	for i := range r.NICs {
		n := &r.NICs[i]
		n.Link(p)
	}
}

// Workload
type Workload struct {
	SelfLink string `json:"selfLink"`
	XVM
	Host       *Host      `json:"host"`
	Cluster    Cluster    `json:"cluster"`
	DataCenter DataCenter `json:"dataCenter"`
	ServerCpu  ServerCpu  `json:"serverCpu"`
}

// Build self link (URI).
func (r *Workload) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		WorkloadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VMParam:            r.ID,
		})
	r.XVM.Link(p)
	r.Cluster.Link(p)
	r.DataCenter.Link(p)
	r.ServerCpu.Link(p)
	if r.Host != nil {
		r.Host.Link(p)
	}
}

// Expand the workload.
func (r *Workload) Expand(db libmodel.DB) (err error) {
	// VM
	err = r.XVM.Expand(db)
	if err != nil {
		return err
	}
	// Host
	if r.VM.Host != "" {
		r.Host = &Host{}
		host := &model.Host{
			Base: model.Base{ID: r.VM.Host},
		}
		err = db.Get(host)
		if err != nil {
			return err
		}
		r.Host.With(host)
	}
	// Cluster.
	cluster := &model.Cluster{
		Base: model.Base{ID: r.VM.Cluster},
	}
	err = db.Get(cluster)
	if err != nil {
		return err
	}
	r.Cluster.With(cluster)
	// DataCenter.
	dataCenter := &model.DataCenter{
		Base: model.Base{ID: cluster.DataCenter},
	}
	err = db.Get(dataCenter)
	if err != nil {
		return err
	}
	r.DataCenter.With(dataCenter)
	// Server CPU
	serverCpu := &model.ServerCpu{
		Base: model.Base{ID: strings.Join([]string{cluster.Version.Major, cluster.Version.Minor}, ".")},
	}
	err = db.Get(serverCpu)
	if err != nil {
		return err
	}
	r.ServerCpu.With(serverCpu)

	return
}
