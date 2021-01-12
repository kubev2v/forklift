package ocp

import (
	"github.com/gin-gonic/gin"
	liberr "github.com/konveyor/controller/pkg/error"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/ocp"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	ProviderParam = base.ProviderParam
	ProvidersRoot = Root
	ProviderRoot  = ProvidersRoot + "/:" + ProviderParam
)

//
// Provider handler.
type ProviderHandler struct {
	base.Handler
}

//
// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProvidersRoot, h.List)
	e.GET(ProvidersRoot+"/", h.List)
	e.GET(ProviderRoot, h.Get)
}

//
// List resources in a REST collection.
func (h ProviderHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	content, err := h.ListContent(ctx)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h ProviderHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.Provider.Type() != api.OpenShift {
		ctx.Status(http.StatusNotFound)
		return
	}
	h.Detail = true
	m := &model.Provider{}
	m.With(h.Provider)
	r := Provider{}
	r.With(m)
	err := h.AddCount(&r)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.SelfLink = h.Link(m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build the list content.
func (h *ProviderHandler) ListContent(ctx *gin.Context) (content []interface{}, err error) {
	content = []interface{}{}
	list := h.Container.List()
	ns := ctx.Param(base.NsParam)
	for _, reconciler := range list {
		if p, cast := reconciler.Owner().(*api.Provider); cast {
			if p.Type() != api.OpenShift {
				continue
			}
			if ns != "" && ns != p.Namespace {
				continue
			}
			if reconciler, found := h.Container.Get(p); found {
				h.Reconciler = reconciler
			} else {
				continue
			}
			m := &model.Provider{}
			m.With(p)
			r := Provider{}
			r.With(m)
			aErr := h.AddCount(&r)
			if aErr != nil {
				err = liberr.Wrap(aErr)
				return
			}
			r.SelfLink = h.Link(m)
			content = append(content, r.Content(h.Detail))
		}
	}

	h.Page.Slice(&content)

	return
}

//
// Add counts.
func (h ProviderHandler) AddCount(r *Provider) error {
	if !h.Detail {
		return nil
	}

	//
	// TODO:

	return nil
}

//
// Build self link (URI).
func (h ProviderHandler) Link(m *model.Provider) string {
	return h.Handler.Link(
		ProviderRoot,
		base.Params{
			base.NsParam:  m.Namespace,
			ProviderParam: m.Name,
		})
}

//
// REST Resource.
type Provider struct {
	Resource
	Type           string       `json:"type"`
	Object         api.Provider `json:"object"`
	VMCount        int64        `json:"vmCount"`
	NetworkCount   int64        `json:"networkCount"`
	NamespaceCount int64        `json:"namespaceCount"`
}

//
// Set fields with the specified object.
func (r *Provider) With(m *model.Provider) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Object = m.Object
}

//
// As content.
func (r *Provider) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
