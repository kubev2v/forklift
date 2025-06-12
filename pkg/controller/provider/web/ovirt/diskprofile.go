package ovirt

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	DiskProfileParam      = "profile"
	DiskProfileCollection = "diskprofiles"
	DiskProfilesRoot      = ProviderRoot + "/" + DiskProfileCollection
	DiskProfileRoot       = DiskProfilesRoot + "/:" + DiskProfileParam
)

// DiskProfile handler.
type DiskProfileHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *DiskProfileHandler) AddRoutes(e *gin.Engine) {
	e.GET(DiskProfilesRoot, h.List)
	e.GET(DiskProfilesRoot+"/", h.List)
	e.GET(DiskProfileRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DiskProfileHandler) List(ctx *gin.Context) {
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
	list := []model.DiskProfile{}
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
		r := &DiskProfile{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h DiskProfileHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.DiskProfile{
		Base: model.Base{
			ID: ctx.Param(DiskProfileParam),
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
	r := &DiskProfile{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *DiskProfileHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.DiskProfile{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.DiskProfile)
			profile := &DiskProfile{}
			profile.With(m)
			profile.Link(h.Provider)
			r = profile
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
type DiskProfile struct {
	Resource
	StorageDomain string `json:"storageDomain"`
	QoS           string `json:"qos"`
}

// Build the resource using the model.
func (r *DiskProfile) With(m *model.DiskProfile) {
	r.Resource.With(&m.Base)
	r.StorageDomain = m.StorageDomain
	r.QoS = m.QoS
}

// Build self link (URI).
func (r *DiskProfile) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		DiskProfileRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DiskProfileParam:   r.ID,
		})
}

// As content.
func (r *DiskProfile) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
