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
	NICProfileParam      = "profile"
	NICProfileCollection = "nicprofiles"
	NICProfilesRoot      = ProviderRoot + "/" + NICProfileCollection
	NICProfileRoot       = NICProfilesRoot + "/:" + NICProfileParam
)

//
// NICProfile handler.
type NICProfileHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *NICProfileHandler) AddRoutes(e *gin.Engine) {
	e.GET(NICProfilesRoot, h.List)
	e.GET(NICProfilesRoot+"/", h.List)
	e.GET(NICProfileRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h NICProfileHandler) List(ctx *gin.Context) {
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
	list := []model.NICProfile{}
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
		r := &NICProfile{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h NICProfileHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.NICProfile{
		Base: model.Base{
			ID: ctx.Param(NICProfileParam),
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
	r := &NICProfile{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Watch.
func (h *NICProfileHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.NICProfile{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.NICProfile)
			profile := &NICProfile{}
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

//
// REST Resource.
type NICProfile struct {
	Resource
	Network       string           `json:"network"`
	NetworkFilter string           `json:"networkFilter"`
	PortMirroring bool             `json:"portMirroring"`
	QoS           string           `json:"qos"`
	Properties    []model.Property `json:"properties"`
}

//
// Build the resource using the model.
func (r *NICProfile) With(m *model.NICProfile) {
	r.Resource.With(&m.Base)
	r.Network = m.Network
	r.NetworkFilter = m.NetworkFilter
	r.PortMirroring = m.PortMirroring
	r.QoS = m.QoS
	r.Properties = m.Properties
}

//
// Build self link (URI).
func (r *NICProfile) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		NICProfileRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			NICProfileParam:    r.ID,
		})
}

//
// As content.
func (r *NICProfile) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
