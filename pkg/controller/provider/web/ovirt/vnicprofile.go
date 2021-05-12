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
	VNICProfileParam      = "profile"
	VNICProfileCollection = "vnicprofiles"
	VNICProfilesRoot      = ProviderRoot + "/" + VNICProfileCollection
	VNICProfileRoot       = VNICProfilesRoot + "/:" + VNICProfileParam
)

//
// VNICProfile handler.
type VNICProfileHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *VNICProfileHandler) AddRoutes(e *gin.Engine) {
	e.GET(VNICProfilesRoot, h.List)
	e.GET(VNICProfilesRoot+"/", h.List)
	e.GET(VNICProfileRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h VNICProfileHandler) List(ctx *gin.Context) {
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
	list := []model.VNICProfile{}
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
		r := &VNICProfile{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h VNICProfileHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.VNICProfile{
		Base: model.Base{
			ID: ctx.Param(VNICProfileParam),
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
	r := &VNICProfile{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h VNICProfileHandler) Link(p *api.Provider, m *model.VNICProfile) string {
	return h.Handler.Link(
		VNICProfileRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			VNICProfileParam:   m.ID,
		})
}

//
// Watch.
func (h VNICProfileHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.VNICProfile{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.VNICProfile)
			profile := &VNICProfile{}
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
type VNICProfile struct {
	Resource
	DataCenter string    `json:"dataCenter"`
	QoS        model.Ref `json:"qos"`
}

//
// Build the resource using the model.
func (r *VNICProfile) With(m *model.VNICProfile) {
	r.Resource.With(&m.Base)
	r.DataCenter = m.DataCenter
	r.QoS = m.QoS
}

//
// As content.
func (r *VNICProfile) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
