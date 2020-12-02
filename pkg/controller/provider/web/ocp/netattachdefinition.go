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
	NetworkAttachmentDefinitionParam    = "network"
	NetworkAttachmentDefinitionsRoot    = NamespaceRoot + "/networkattachmentdefinitions"
	AllNetworkAttachmentDefinitionsRoot = ProviderRoot + "/networkattachmentdefinitions"
	NetworkAttachmentDefinitionRoot     = NetworkAttachmentDefinitionsRoot + "/:" +
		NetworkAttachmentDefinitionParam
)

//
// NetworkAttachmentDefinition handler.
type NetworkAttachmentDefinitionHandler struct {
	Handler
}

//
// Add routes to the `gin` router.
func (h *NetworkAttachmentDefinitionHandler) AddRoutes(e *gin.Engine) {
	e.GET(AllNetworkAttachmentDefinitionsRoot, h.ListAll)
	e.GET(NetworkAttachmentDefinitionsRoot, h.List)
	e.GET(NetworkAttachmentDefinitionsRoot+"/", h.List)
	e.GET(NetworkAttachmentDefinitionRoot, h.Get)
}

//
// List resources in a REST collection (all namespaces).
func (h NetworkAttachmentDefinitionHandler) ListAll(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.NetworkAttachmentDefinition{}
	err := db.List(
		&list,
		libmodel.ListOptions{
			Page: &h.Page,
		})
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
// List resources in a REST collection.
func (h NetworkAttachmentDefinitionHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	list := []model.NetworkAttachmentDefinition{}
	err := db.List(
		&list,
		libmodel.ListOptions{
			Predicate: libmodel.Eq("Namespace", ctx.Param(Ns2Param)),
			Page:      &h.Page,
		})
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
func (h NetworkAttachmentDefinitionHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.NetworkAttachmentDefinition{
		Base: model.Base{
			Namespace: ctx.Param(Ns2Param),
			Name:      ctx.Param(NetworkAttachmentDefinitionParam),
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
func (h NetworkAttachmentDefinitionHandler) Link(p *api.Provider, m *model.NetworkAttachmentDefinition) string {
	return h.Handler.Link(
		NetworkAttachmentDefinitionRoot,
		base.Params{
			base.NsParam:                     p.Namespace,
			base.ProviderParam:               p.Name,
			Ns2Param:                         m.Namespace,
			NetworkAttachmentDefinitionParam: m.Name,
		})
}

//
// REST Resource.
type NetworkAttachmentDefinition struct {
	Resource
	Object interface{} `json:"object"`
}

//
// Set fields with the specified object.
func (r *NetworkAttachmentDefinition) With(m *model.NetworkAttachmentDefinition) {
	r.Resource.With(&m.Base)
	r.Object = m.DecodeObject(&net.NetworkAttachmentDefinition{})
}

//
// As content.
func (r *NetworkAttachmentDefinition) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
