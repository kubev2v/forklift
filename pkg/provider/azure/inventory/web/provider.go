package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	ocpmodel "github.com/kubev2v/forklift/pkg/controller/provider/model/ocp"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ocp"
	"github.com/kubev2v/forklift/pkg/provider/azure/inventory/model"
)

type ProviderHandler struct {
	base.Handler
}

func (h *ProviderHandler) Prepare(ctx *gin.Context) (int, error) {
	return h.Handler.Prepare(ctx)
}

func (h *ProviderHandler) AddRoutes(e *gin.Engine) {
	e.GET(Root, h.List)
	e.GET(Root+"/", h.List)
	e.GET(ProviderRoot, h.Get)
}

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

func (h *ProviderHandler) ListContent(ctx *gin.Context) (content []interface{}, err error) {
	content = []interface{}{}
	q := ctx.Request.URL.Query()
	ns := q.Get(base.NsParam)
	for _, collector := range h.Container.List() {
		p, cast := collector.Owner().(*api.Provider)
		if !cast {
			continue
		}
		if p.Type() != api.Azure || (ns != "" && ns != p.Namespace) {
			continue
		}
		var found bool
		h.Collector, found = h.Container.Get(p)
		if !found {
			continue
		}
		m := &ocpmodel.Provider{}
		m.With(p)
		r := &ProviderResource{}
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

func (h ProviderHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.Provider.Type() != api.Azure {
		ctx.Status(http.StatusNotFound)
		return
	}
	var found bool
	h.Collector, found = h.Container.Get(h.Provider)
	if !found {
		ctx.Status(http.StatusNotFound)
		return
	}
	m := &ocpmodel.Provider{}
	m.With(h.Provider)
	r := &ProviderResource{}
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

type ProviderResource struct {
	ocp.Resource
	Type         string       `json:"type"`
	Object       api.Provider `json:"object"`
	APIVersion   string       `json:"apiVersion"`
	Product      string       `json:"product"`
	VMCount      int64        `json:"vmCount"`
	NetworkCount int64        `json:"networkCount"`
	DiskCount    int64        `json:"diskCount"`
	StorageCount int64        `json:"storageCount"`
}

func (r *ProviderResource) With(m *ocpmodel.Provider) {
	r.Resource.With(&m.Base)
	r.Type = m.Type
	r.Object = m.Object
	r.APIVersion = "v1beta1"
	r.Product = "Azure"
}

func (r *ProviderResource) Link() {
	r.SelfLink = base.Link(
		ProviderRoot,
		base.Params{
			base.ProviderParam: r.UID,
		})
}

func (r *ProviderResource) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}
	return r
}

func (h ProviderHandler) AddDerived(r *ProviderResource) (err error) {
	var n int64
	if h.Detail == 0 {
		return
	}
	db := h.Collector.DB()

	n, err = db.Count(&model.VM{}, nil)
	if err != nil {
		return
	}
	r.VMCount = n

	n, err = db.Count(&model.Network{}, nil)
	if err != nil {
		return
	}
	r.NetworkCount = n

	n, err = db.Count(&model.Disk{}, nil)
	if err != nil {
		return
	}
	r.DiskCount = n

	n, err = db.Count(&model.Storage{}, nil)
	if err != nil {
		return
	}
	r.StorageCount = n

	return
}
