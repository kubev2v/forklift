package hyperv

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/hyperv"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
)

// Routes.
const (
	HostParam      = "host"
	HostCollection = "hosts"
	HostsRoot      = ProviderRoot + "/" + HostCollection
	HostRoot       = HostsRoot + "/:" + HostParam
)

// Host handler.
type HostHandler struct {
	Handler
}

func (h *HostHandler) AddRoutes(e *gin.Engine) {
	e.GET(HostsRoot, h.List)
	e.GET(HostsRoot+"/", h.List)
	e.GET(HostRoot, h.Get)
}

func (h HostHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	db := h.Collector.DB()
	list := []model.Host{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Host{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	ctx.JSON(http.StatusOK, content)
}

func (h HostHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Host{
		Base: model.Base{
			ID: ctx.Param(HostParam),
		},
	}
	db := h.Collector.DB()
	err := db.Get(m)
	if errors.Is(err, model.NotFound) {
		ctx.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	r := &Host{}
	r.With(m)
	r.Link(h.Provider)
	ctx.JSON(http.StatusOK, r.Content(model.MaxDetail))
}

func (h *HostHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Host{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Host)
			host := &Host{}
			host.With(m)
			host.Link(h.Provider)
			r = host
			return
		})
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

// REST Resource.
type Host struct {
	Resource
	Cluster     string      `json:"cluster"`
	State       string      `json:"state"`
	CpuSockets  int16       `json:"cpuSockets"`
	CpuCores    int16       `json:"cpuCores"`
	MemoryBytes int64       `json:"memoryBytes"`
	Networks    []model.Ref `json:"networks"`
	VMs         []model.Ref `json:"vms,omitempty"`
}

func (r *Host) With(m *model.Host) {
	r.Resource.With(&m.Base)
	r.Cluster = m.Cluster
	r.State = m.State
	r.CpuSockets = m.CpuSockets
	r.CpuCores = m.CpuCores
	r.MemoryBytes = m.MemoryBytes
	r.Networks = m.Networks
}

func (r *Host) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		HostRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			HostParam:          r.ID,
		})
}

func (r *Host) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}
	return r
}
