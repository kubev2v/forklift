package ocp

import (
	"errors"
	"github.com/gin-gonic/gin"
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	NadParam = "network"
	NadsRoot = ProviderRoot + "/networkattachmentdefinitions"
	NadRoot  = NadsRoot + "/:" + NadParam
)

//
// NetworkAttachmentDefinition handler.
type NadHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *NadHandler) AddRoutes(e *gin.Engine) {
	e.GET(NadsRoot, h.List)
	e.GET(NadsRoot+"/", h.List)
	e.GET(NadRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h NadHandler) List(ctx *gin.Context) {
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
	list := []model.NetworkAttachmentDefinition{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &NetworkAttachmentDefinition{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h NadHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.NetworkAttachmentDefinition{
		Base: model.Base{
			PK: ctx.Param(NadParam),
		},
	}
	db := h.Reconciler.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &NetworkAttachmentDefinition{}
	r.With(m)
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h NadHandler) Link(p *api.Provider, m *model.NetworkAttachmentDefinition) string {
	return h.Handler.Link(
		NadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			NadParam:           m.PK,
		})
}

//
// Watch.
func (h NadHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.NetworkAttachmentDefinition{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.NetworkAttachmentDefinition)
			nad := &NetworkAttachmentDefinition{}
			nad.With(m)
			nad.SelfLink = h.Link(h.Provider, m)
			r = nad
			return
		})
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
	}
}

//
// REST Resource.
type NetworkAttachmentDefinition struct {
	Resource
	Object net.NetworkAttachmentDefinition `json:"object"`
}

//
// Set fields with the specified object.
func (r *NetworkAttachmentDefinition) With(m *model.NetworkAttachmentDefinition) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

//
// As content.
func (r *NetworkAttachmentDefinition) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
