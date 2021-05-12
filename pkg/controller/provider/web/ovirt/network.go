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
	NetworkParam      = "network"
	NetworkCollection = "networks"
	NetworksRoot      = ProviderRoot + "/" + NetworkCollection
	NetworkRoot       = NetworksRoot + "/:" + NetworkParam
)

//
// Network handler.
type NetworkHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *NetworkHandler) AddRoutes(e *gin.Engine) {
	e.GET(NetworksRoot, h.List)
	e.GET(NetworksRoot+"/", h.List)
	e.GET(NetworkRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h NetworkHandler) List(ctx *gin.Context) {
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
	list := []model.Network{}
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
		r := &Network{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h NetworkHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Network{
		Base: model.Base{
			ID: ctx.Param(NetworkParam),
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
	r := &Network{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h NetworkHandler) Link(p *api.Provider, m *model.Network) string {
	return h.Handler.Link(
		NetworkRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			NetworkParam:       m.ID,
		})
}

//
// Watch.
func (h NetworkHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Network{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Network)
			network := &Network{}
			network.With(m)
			network.SelfLink = h.Link(h.Provider, m)
			r = network
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
type Network struct {
	Resource
	DataCenter   string      `json:"dataCenter"`
	VLan         model.Ref   `json:"vlan"`
	Usages       []string    `json:"usages"`
	VNICProfiles []model.Ref `json:"vnicProfiles"`
}

//
// Build the resource using the model.
func (r *Network) With(m *model.Network) {
	r.Resource.With(&m.Base)
	r.DataCenter = m.DataCenter
	r.VLan = m.VLan
	r.Usages = m.Usages
	r.VNICProfiles = m.VNICProfiles
}

//
// As content.
func (r *Network) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
