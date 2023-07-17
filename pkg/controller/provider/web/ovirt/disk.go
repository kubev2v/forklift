package ovirt

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
)

// Routes
const (
	DiskParam      = "disk"
	DiskCollection = "disks"
	DisksRoot      = ProviderRoot + "/" + DiskCollection
	DiskRoot       = DisksRoot + "/:" + DiskParam
)

// Disk handler.
type DiskHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *DiskHandler) AddRoutes(e *gin.Engine) {
	e.GET(DisksRoot, h.List)
	e.GET(DisksRoot+"/", h.List)
	e.GET(DiskRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DiskHandler) List(ctx *gin.Context) {
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
	db := h.Collector.DB()
	list := []model.Disk{}
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Disk{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h DiskHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	h.Detail = model.MaxDetail
	m := &model.Disk{
		Base: model.Base{
			ID: ctx.Param(DiskParam),
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
	r := &Disk{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(h.Detail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *DiskHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Disk{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Disk)
			disk := &Disk{}
			disk.With(m)
			disk.Link(h.Provider)
			r = disk
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

// REST Resource.
type Disk struct {
	Resource
	Shared          bool   `json:"shared"`
	StorageDomain   string `json:"storageDomain"`
	Profile         string `json:"profile"`
	ProvisionedSize int64  `json:"provisionedSize"`
	ActualSize      int64  `json:"actualSize"`
	StorageType     string `json:"storageType"`
	Status          string `json:"status"`
	Lun             Lun    `json:"lunStorage"`
}

type Lun = model.Lun

// Build the resource using the model.
func (r *Disk) With(m *model.Disk) {
	r.Resource.With(&m.Base)
	r.Profile = m.Profile
	r.Status = m.Status
	r.StorageType = m.StorageType
	r.ProvisionedSize = m.ProvisionedSize
	r.ActualSize = m.ActualSize
	r.Shared = m.Shared
	r.StorageDomain = m.StorageDomain
	r.Lun = m.Lun
}

// Build self link (URI).
func (r *Disk) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		DiskRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DiskParam:          r.ID,
		})
}

// As content.
func (r *Disk) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
