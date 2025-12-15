package ovfbase

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	ovfmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ovf"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
)

// Routes.
const (
	ProviderParam = base.ProviderParam
)

// Provider handler.
type ProviderHandler struct {
	base.Handler
	Config Config
}

// Add routes to the `gin` router.
func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(h.Config.ProvidersRoot(), h.List)
	e.GET(h.Config.ProvidersRoot()+"/", h.List)
	e.GET(h.Config.ProviderRoot(), h.Get)
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
	if h.Provider.Type() != h.Config.ProviderType {
		ctx.Status(http.StatusNotFound)
		return
	}
	h.Detail = model.MaxDetail
	m := &model.Provider{}
	m.With(h.Provider)
	r := Provider{}
	r.With(m)
	r.Config = h.Config
	err = h.AddDerived(&r)
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
			if p.Type() != h.Config.ProviderType || (ns != "" && ns != p.Namespace) {
				continue
			}
			collector, found := h.Container.Get(p)
			if !found {
				continue
			}
			h.Collector = collector
			m := &model.Provider{}
			m.With(p)
			r := Provider{}
			r.With(m)
			r.Config = h.Config
			aErr := h.AddDerived(&r)
			if aErr != nil {
				err = aErr
				return
			}
			r.Link()
			content = append(content, r.Content(h.Detail))
		}
	}

	h.Page.Slice(&content)

	return
}

// Add derived fields.
func (h ProviderHandler) AddDerived(r *Provider) (err error) {
	var n int64
	if h.Detail == 0 {
		return
	}
	db := h.Collector.DB()
	// VM
	n, err = db.Count(&ovfmodel.VM{}, nil)
	if err != nil {
		return
	}
	r.VMCount = n
	// Network
	n, err = db.Count(&ovfmodel.Network{}, nil)
	if err != nil {
		return
	}
	r.NetworkCount = n
	// Disk
	n, err = db.Count(&ovfmodel.Disk{}, nil)
	if err != nil {
		return
	}
	r.DiskCount = n
	// Storage count
	n, err = db.Count(&ovfmodel.Storage{}, nil)
	if err != nil {
		return
	}
	r.StorageCount = n

	return
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
	DiskCount    int64        `json:"diskCount"`
	StorageCount int64        `json:"storageCount"`
	Config       Config       `json:"-"`
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
		r.Config.ProviderRoot(),
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
