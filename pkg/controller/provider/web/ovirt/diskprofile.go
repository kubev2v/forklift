package ovirt

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ovirt"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	DiskProfileParam      = "profile"
	DiskProfileCollection = "diskprofiles"
	DiskProfilesRoot      = ProviderRoot + "/" + DiskProfileCollection
	DiskProfileRoot       = DiskProfilesRoot + "/:" + DiskProfileParam
)

//
// DiskProfile handler.
type DiskProfileHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *DiskProfileHandler) AddRoutes(e *gin.Engine) {
	e.GET(DiskProfilesRoot, h.List)
	e.GET(DiskProfilesRoot+"/", h.List)
	e.GET(DiskProfileRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h DiskProfileHandler) List(ctx *gin.Context) {
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
	list := []model.DiskProfile{}
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
		r := &DiskProfile{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h DiskProfileHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.DiskProfile{
		Base: model.Base{
			ID: ctx.Param(DiskProfileParam),
		},
	}
	db := h.Reconciler.DB()
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
	r := &DiskProfile{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h DiskProfileHandler) Link(p *api.Provider, m *model.DiskProfile) string {
	return h.Handler.Link(
		DiskProfileRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			DiskProfileParam:   m.ID,
		})
}

//
// Watch.
func (h DiskProfileHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.DiskProfile{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.DiskProfile)
			profile := &DiskProfile{}
			profile.With(m)
			profile.SelfLink = h.Link(h.Provider, m)
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

//
// REST Resource.
type DiskProfile struct {
	Resource
	StorageDomain string `json:"storageDomain"`
	QoS           string `json:"qos"`
}

//
// Build the resource using the model.
func (r *DiskProfile) With(m *model.DiskProfile) {
	r.Resource.With(&m.Base)
	r.StorageDomain = m.StorageDomain
	r.QoS = m.QoS
}

//
// As content.
func (r *DiskProfile) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
