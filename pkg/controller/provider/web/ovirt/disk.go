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
)

//
// Routes
const (
	DiskParam      = "disk"
	DiskCollection = "disks"
	DisksRoot      = ProviderRoot + "/" + DiskCollection
	DiskRoot       = DisksRoot + "/:" + DiskParam
)

//
// Disk handler.
type DiskHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *DiskHandler) AddRoutes(e *gin.Engine) {
	e.GET(DisksRoot, h.List)
	e.GET(DisksRoot+"/", h.List)
	e.GET(DiskRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DiskHandler) List(ctx *gin.Context) {
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
	list := []model.Disk{}
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
	for _, m := range list {
		r := &Disk{}
		r.With(&m)
		err = r.Expand(h.Collector.DB())
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
func (h DiskHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	h.Detail = true
	m := &model.Disk{
		Base: model.Base{
			ID: ctx.Param(DiskParam),
		},
	}
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
	r := &Disk{}
	r.With(m)
	err = r.Expand(h.Collector.DB())
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
func (h *DiskHandler) Expand(r *Disk) (err error) {
	if !h.Detail {
		return
	}
	err = r.Expand(h.Collector.DB())
	return
}

//
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

//
// REST Resource.
type Disk struct {
	Resource
	Shared          bool        `json:"shared"`
	StorageDomain   string      `json:"storageDomain"`
	Profile         DiskProfile `json:"profile"`
	ProvisionedSize int64       `json:"provisionedSize"`
	ActualSize      int64       `json:"actualSize"`
	StorageType     string      `json:"storageType"`
	Status          string      `json:"status"`
}

//
// Build the resource using the model.
func (r *Disk) With(m *model.Disk) {
	r.Resource.With(&m.Base)
	r.Status = m.Status
	r.StorageType = m.StorageType
	r.ProvisionedSize = m.ProvisionedSize
	r.ActualSize = m.ActualSize
	r.Shared = m.Shared
	r.StorageDomain = m.StorageDomain
	r.Profile = DiskProfile{
		Resource: Resource{
			ID: m.Profile,
		},
	}
}

//
// Expand the resource.
// The profile.ID is optional.
func (r *Disk) Expand(db libmodel.DB) (err error) {
	if r.Profile.ID == "" {
		return
	}
	defer func() {
		if err != nil {
			err = liberr.Wrap(err, "disk", r.ID)
		}
	}()
	profile := &model.DiskProfile{
		Base: model.Base{ID: r.Profile.ID},
	}
	err = db.Get(profile)
	if err != nil {
		return
	}
	r.Profile.With(profile)
	return
}

//
// Build self link (URI).
func (r *Disk) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		DiskRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DiskParam:          r.ID,
		})
	r.Profile.Link(p)
}

//
// As content.
func (r *Disk) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
