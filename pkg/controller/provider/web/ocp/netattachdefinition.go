package ocp

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	net "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
)

// Routes.
const (
	NadParam = "network"
	NadsRoot = ProviderRoot + "/networkattachmentdefinitions"
	NadRoot  = NadsRoot + "/:" + NadParam
)

// NetworkAttachmentDefinition handler.
type NadHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *NadHandler) AddRoutes(e *gin.Engine) {
	e.GET(NadsRoot, h.List)
	e.GET(NadsRoot+"/", h.List)
	e.GET(NadRoot, h.Get)
}

// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
func (h NadHandler) List(ctx *gin.Context) {
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
	list := []model.NetworkAttachmentDefinition{}
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
		r := &NetworkAttachmentDefinition{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h NadHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.NetworkAttachmentDefinition{
		Base: model.Base{
			UID: ctx.Param(NadParam),
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
	r := &NetworkAttachmentDefinition{}
	r.With(m)
	r.Link(h.Provider)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h NadHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.NetworkAttachmentDefinition{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.NetworkAttachmentDefinition)
			nad := &NetworkAttachmentDefinition{}
			nad.With(m)
			nad.Link(h.Provider)
			r = nad
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
type NetworkAttachmentDefinition struct {
	Resource
	Object net.NetworkAttachmentDefinition `json:"object"`
}

// Set fields with the specified object.
func (r *NetworkAttachmentDefinition) With(m *model.NetworkAttachmentDefinition) {
	r.Resource.With(&m.Base)
	r.Object = m.Object
}

// Build self link (URI).
func (r *NetworkAttachmentDefinition) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		NadRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			NadParam:           r.UID,
		})
}

// As content.
func (r *NetworkAttachmentDefinition) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
