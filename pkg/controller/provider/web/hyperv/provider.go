package hyperv

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	ocpmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
)

// Routes.
const (
	ProvidersRoot = Root
	ProviderRoot  = ProvidersRoot + "/:" + ProviderParam
)

// Provider handler.
type ProviderHandler struct {
	base.Handler
	Config Config
}

// Prepare to handle the request.
func (h *ProviderHandler) Prepare(ctx *gin.Context) (int, error) {
	return h.Handler.Prepare(ctx)
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
	if h.WatchRequest {
		ctx.Status(http.StatusBadRequest)
		return
	}
	content, err := h.ListContent(ctx)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	ctx.JSON(http.StatusOK, content)
}

// ListContent returns list of providers for aggregation.
func (h *ProviderHandler) ListContent(ctx *gin.Context) (content []interface{}, err error) {
	content = []interface{}{}
	q := ctx.Request.URL.Query()
	ns := q.Get(base.NsParam)
	for _, collector := range h.Container.List() {
		p, cast := collector.Owner().(*api.Provider)
		if !cast {
			continue
		}
		if p.Type() != h.Config.ProviderType || (ns != "" && ns != p.Namespace) {
			continue
		}
		var found bool
		h.Collector, found = h.Container.Get(p)
		if !found {
			continue
		}
		m := &ocpmodel.Provider{}
		m.With(p)
		r := &Provider{}
		r.With(m)
		r.Link()
		aErr := h.AddDerived(r)
		if aErr != nil {
			err = aErr
			return
		}
		content = append(content, r.Content(model.MaxDetail))
	}
	return
}

// Get a specific REST resource.
func (h ProviderHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.Provider.Type() != api.HyperV {
		ctx.Status(http.StatusNotFound)
		return
	}
	h.Collector, _ = h.Container.Get(h.Provider)
	m := &ocpmodel.Provider{}
	m.With(h.Provider)
	r := &Provider{}
	r.With(m)
	r.Link()
	err = h.AddDerived(r)
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// REST Resource.
type Provider struct {
	ocp.Resource
	Type         string       `json:"type"`
	Object       api.Provider `json:"object"`
	APIVersion   string       `json:"apiVersion"`
	Product      string       `json:"product"`
	VMCount      int64        `json:"vmCount"`
	NetworkCount int64        `json:"networkCount"`
	StorageCount int64        `json:"storageCount"`
}

// Set fields with the specified object.
func (r *Provider) With(m *ocpmodel.Provider) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Object = m.Object
	r.APIVersion = "v1beta1"
	r.Product = "HyperV"
}

// Build self link (URI).
func (r *Provider) Link() {
	r.SelfLink = base.Link(
		ProviderRoot,
		base.Params{
			base.ProviderParam: r.UID,
		})
}

// As content.
func (r *Provider) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}
	return r
}

// Add derived fields
func (h ProviderHandler) AddDerived(r *Provider) (err error) {
	var n int64
	if h.Detail == 0 {
		return
	}
	db := h.Collector.DB()

	// VM count
	n, err = db.Count(&model.VM{}, nil)
	if err != nil {
		return
	}
	r.VMCount = n

	// Network count
	n, err = db.Count(&model.Network{}, nil)
	if err != nil {
		return
	}
	r.NetworkCount = n

	// Storage count
	n, err = db.Count(&model.Storage{}, nil)
	if err != nil {
		return
	}
	r.StorageCount = n

	return
}
