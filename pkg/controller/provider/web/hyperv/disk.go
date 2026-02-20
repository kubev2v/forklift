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
		r := &Disk{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h DiskHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
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
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &Disk{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

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
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

// Filter result set.
func (h *DiskHandler) filter(ctx *gin.Context, list *[]model.Disk) (err error) {
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
	kept := []model.Disk{}
	for _, m := range *list {
		if h.PathMatchRoot(m.Name, name) {
			kept = append(kept, m)
		}
	}

	*list = kept
	return
}

// REST Resource.
type Disk struct {
	Resource
	WindowsPath string `json:"windowsPath,omitempty"`
	SMBPath     string `json:"smbPath,omitempty"`
	Capacity    int64  `json:"capacity"`
	Format      string `json:"format,omitempty"`
	RCTEnabled  bool   `json:"rctEnabled"`
	Datastore   string `json:"datastore"`
}

// Build the resource using the model.
func (r *Disk) With(m *model.Disk) {
	r.Resource.With(&m.Base)
	r.WindowsPath = m.WindowsPath
	r.SMBPath = m.SMBPath
	r.Capacity = m.Capacity
	r.Format = m.Format
	r.RCTEnabled = m.RCTEnabled
	r.Datastore = m.Datastore.ID
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
