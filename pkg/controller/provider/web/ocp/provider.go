package ocp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
)

// Routes.
const (
	ProviderParam = base.ProviderParam
	ProvidersRoot = Root
	ProviderRoot  = ProvidersRoot + "/:" + ProviderParam
)

// Provider handler.
type ProviderHandler struct {
	Handler
}

// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(ProvidersRoot, h.List)
	e.GET(ProvidersRoot+"/", h.List)
	e.GET(ProviderRoot, h.Get)
}

// List resources in a REST collection.
func (h ProviderHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	content, err := h.ListContent(ctx)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ProviderHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.Provider.Type() != api.OpenShift {
		ctx.Status(http.StatusNotFound)
		return
	}
	h.Detail = model.MaxDetail
	m := &model.Provider{}
	m.With(h.Provider)
	r := Provider{}
	r.With(m)
	err = h.AddCount(ctx, &r)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r.Link()
	content := r.Content(h.Detail)

	ctx.JSON(http.StatusOK, content)
}

// Build the list content.
func (h *ProviderHandler) ListContent(ctx *gin.Context) (content []interface{}, err error) {
	content = []interface{}{}
	list := h.Container.List()
	q := ctx.Request.URL.Query()
	ns := q.Get(base.NsParam)
	for _, collector := range list {
		if p, cast := collector.Owner().(*api.Provider); cast {
			if p.Type() != api.OpenShift {
				continue
			}
			if ns != "" && ns != p.Namespace {
				continue
			}
			if collector, found := h.Container.Get(p); found {
				h.Collector = collector
			} else {
				continue
			}
			m := &model.Provider{}
			m.With(p)
			r := Provider{}
			r.With(m)
			aErr := h.AddCount(ctx, &r)
			if aErr != nil {
				log.Error(aErr, "Failed to get count for provider", "provider", p.Name, "url", ctx.Request.URL)
				continue
			}
			r.Link()
			content = append(content, r.Content(h.Detail))
		}
	}

	h.Page.Slice(&content)

	return
}

// Add counts.
func (h ProviderHandler) AddCount(ctx *gin.Context, r *Provider) (err error) {
	if h.Detail == 0 {
		return nil
	}

	// VM
	vms, err := h.VMs(ctx, &r.Object)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.VMCount = int64(len(vms))

	// Network
	nets, err := h.NetworkAttachmentDefinitions(ctx)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.NetworkCount = int64(len(nets)) + 1

	// StorageClass
	storageclasses, err := h.StorageClasses(ctx)
	if err != nil {
		return liberr.Wrap(err)
	}
	r.StorageClassCount = int64(len(storageclasses))

	return nil
}

// REST Resource.
type Provider struct {
	Resource
	Type              string       `json:"type"`
	Object            api.Provider `json:"object"`
	VMCount           int64        `json:"vmCount"`
	NetworkCount      int64        `json:"networkCount"`
	StorageClassCount int64        `json:"storageClassCount"`
}

// Set fields with the specified object.
func (r *Provider) With(m *model.Provider) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Object = m.Object
}

// Build self link (URI).
func (r *Provider) Link() {
	r.SelfLink = base.Link(
		ProviderRoot,
		base.Params{
			ProviderParam: r.UID,
		})
}

// As content.
func (r *Provider) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}

	return r
}
