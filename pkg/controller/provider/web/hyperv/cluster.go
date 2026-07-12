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
	ClusterParam      = "cluster"
	ClusterCollection = "clusters"
	ClustersRoot      = ProviderRoot + "/" + ClusterCollection
	ClusterRoot       = ClustersRoot + "/:" + ClusterParam
)

// Cluster handler.
type ClusterHandler struct {
	Handler
}

func (h *ClusterHandler) AddRoutes(e *gin.Engine) {
	e.GET(ClustersRoot, h.List)
	e.GET(ClustersRoot+"/", h.List)
	e.GET(ClusterRoot, h.Get)
}

func (h ClusterHandler) List(ctx *gin.Context) {
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
	list := []model.Cluster{}
	err := db.List(&list, h.ListOptions(ctx))
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Cluster{}
		r.With(&m)
		r.Link(h.Provider)
		content = append(content, r.Content(h.Detail))
	}
	ctx.JSON(http.StatusOK, content)
}

func (h ClusterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	m := &model.Cluster{
		Base: model.Base{
			ID: ctx.Param(ClusterParam),
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
	r := &Cluster{}
	r.With(m)
	r.Link(h.Provider)
	ctx.JSON(http.StatusOK, r.Content(model.MaxDetail))
}

func (h *ClusterHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Cluster{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Cluster)
			cluster := &Cluster{}
			cluster.With(m)
			cluster.Link(h.Provider)
			r = cluster
			return
		})
	if err != nil {
		log.Trace(err, "url", ctx.Request.URL)
		ctx.Status(http.StatusInternalServerError)
	}
}

// REST Resource.
type Cluster struct {
	Resource
	Domain string      `json:"domain"`
	Nodes  []model.Ref `json:"nodes"`
}

func (r *Cluster) With(m *model.Cluster) {
	r.Resource.With(&m.Base)
	r.Domain = m.Domain
	r.Nodes = m.Nodes
}

func (r *Cluster) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ClusterRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ClusterParam:       r.ID,
		})
}

func (r *Cluster) Content(detail int) interface{} {
	if detail == 0 {
		return r.Resource
	}
	return r
}
