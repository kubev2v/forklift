package ovirt

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
)

// Routes.
const (
	ProviderParam = base.ProviderParam
	ProvidersRoot = Root
	ProviderRoot  = ProvidersRoot + "/:" + ProviderParam
)

// Provider handler.
type ProviderHandler struct {
	base.Handler
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
	if h.Provider.Type() != api.OVirt {
		ctx.Status(http.StatusNotFound)
		return
	}
	h.Detail = model.MaxDetail
	m := &model.Provider{}
	m.With(h.Provider)
	r := Provider{}
	r.With(m)
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
			if p.Type() != api.OVirt || (ns != "" && ns != p.Namespace) {
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
func (h *ProviderHandler) AddDerived(r *Provider) (err error) {
	var n int64
	if h.Detail == 0 {
		return
	}
	db := h.Collector.DB()
	// DataCenter
	n, err = db.Count(&ovirt.DataCenter{}, nil)
	if err != nil {
		return
	}
	r.DatacenterCount = n
	// Cluster
	n, err = db.Count(&ovirt.Cluster{}, nil)
	if err != nil {
		return
	}
	r.ClusterCount = n
	// Host
	n, err = db.Count(&ovirt.Host{}, nil)
	if err != nil {
		return
	}
	r.HostCount = n
	// VM
	n, err = db.Count(&ovirt.VM{}, nil)
	if err != nil {
		return
	}
	r.VMCount = n
	// Network
	n, err = db.Count(&ovirt.Network{}, nil)
	if err != nil {
		return
	}
	r.NetworkCount = n
	// StorageDomain
	n, err = db.Count(&ovirt.StorageDomain{}, nil)
	if err != nil {
		return
	}
	r.StorageDomainCount = n

	return
}

// REST Resource.
type Provider struct {
	ocp.Resource
	Type               string       `json:"type"`
	Object             api.Provider `json:"object"`
	DatacenterCount    int64        `json:"datacenterCount"`
	ClusterCount       int64        `json:"clusterCount"`
	HostCount          int64        `json:"hostCount"`
	VMCount            int64        `json:"vmCount"`
	NetworkCount       int64        `json:"networkCount"`
	StorageDomainCount int64        `json:"storageDomainCount"`
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
