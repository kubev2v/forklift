package vsphere

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1alpha1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	ClusterParam      = "cluster"
	ClusterCollection = "clusters"
	ClustersRoot      = ProviderRoot + "/" + ClusterCollection
	ClusterRoot       = ClustersRoot + "/:" + ClusterParam
)

//
// Cluster handler.
type ClusterHandler struct {
	Handler
	// Selected cluster.
	cluster *model.Cluster
}

//
// Add routes to the `gin` router.
func (h *ClusterHandler) AddRoutes(e *gin.Engine) {
	e.GET(ClustersRoot, h.List)
	e.GET(ClustersRoot+"/", h.List)
	e.GET(ClusterRoot, h.Get)
}

//
// List resources in a REST collection.
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
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
	db := h.Reconciler.DB()
	list := []model.Cluster{}
	err := db.List(&list, h.ListOptions(ctx))
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
		r := &Cluster{}
		r.With(&m)
		r.SelfLink = h.Link(h.Provider, &m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
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
	db := h.Reconciler.DB()
	err := db.Get(m)
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
	r := &Cluster{}
	r.With(m)
	r.Path, err = m.Path(db)
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		return
	}
	r.SelfLink = h.Link(h.Provider, m)
	content := r.Content(true)

	ctx.JSON(http.StatusOK, content)
}

//
// Build self link (URI).
func (h ClusterHandler) Link(p *api.Provider, m *model.Cluster) string {
	return h.Handler.Link(
		ClusterRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ClusterParam:       m.ID,
		})
}

//
// Watch.
func (h ClusterHandler) watch(ctx *gin.Context) {
	db := h.Reconciler.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Cluster{},
		func(in libmodel.Model) (r interface{}) {
			m := in.(*model.Cluster)
			cluster := &Cluster{}
			cluster.With(m)
			cluster.SelfLink = h.Link(h.Provider, m)
			cluster.Path, _ = m.Path(db)
			r = cluster
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

//
// REST Resource.
type Cluster struct {
	Resource
	Networks    []model.Ref `json:"networks"`
	Datastores  []model.Ref `json:"datastores"`
	Hosts       []model.Ref `json:"hosts"`
	DasEnabled  bool        `json:"dasEnabled"`
	DasVms      []model.Ref `json:"dasVms"`
	DrsEnabled  bool        `json:"drsEnabled"`
	DrsBehavior string      `json:"drsBehavior"`
	DrsVms      []model.Ref `json:"drsVms"`
}

//
// Build the resource using the model.
func (r *Cluster) With(m *model.Cluster) {
	r.Resource.With(&m.Base)
	r.DasEnabled = m.DasEnabled
	r.DrsEnabled = m.DrsEnabled
	r.DrsBehavior = m.DrsBehavior
	r.Networks = m.Networks
	r.Datastores = m.Datastores
	r.Hosts = m.Hosts
	r.DasVms = m.DasVms
	r.DrsVms = m.DasVms
}

//
// As content.
func (r *Cluster) Content(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
