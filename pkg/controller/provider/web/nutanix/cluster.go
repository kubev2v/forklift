package nutanix

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/nutanix"
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

// Add routes to the `gin` router.
func (h *ClusterHandler) AddRoutes(e *gin.Engine) {
	e.GET(ClustersRoot, h.List)
	e.GET(ClustersRoot+"/", h.List)
	e.GET(ClusterRoot, h.Get)
}

// List resources in a REST collection.
func (h ClusterHandler) List(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	if h.WatchRequest {
		h.watch(ctx)
		return
	}
	defer func() {
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
		}
	}()
	db := h.Collector.DB()
	list := []model.Cluster{}
	err = db.List(&list, h.ListOptions(ctx))
	if err != nil {
		return
	}
	content := []interface{}{}
	err = h.filter(ctx, &list)
	if err != nil {
		return
	}
	pb := PathBuilder{DB: db}
	for _, m := range list {
		r := &Cluster{}
		r.With(&m)
		r.Link(h.Provider)
		r.Path = pb.Path(&m)
		content = append(content, r.Content(h.Detail))
	}

	ctx.JSON(http.StatusOK, content)
}

// Get a specific REST resource.
func (h ClusterHandler) Get(ctx *gin.Context) {
	status, err := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return
	}
	m := &model.Cluster{
		Base: model.Base{
			ID: ctx.Param(ClusterParam),
		},
	}
	db := h.Collector.DB()
	err = db.Get(m)
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
	pb := PathBuilder{DB: db}
	r := &Cluster{}
	r.With(m)
	r.Link(h.Provider)
	r.Path = pb.Path(m)
	content := r.Content(model.MaxDetail)

	ctx.JSON(http.StatusOK, content)
}

// Watch.
func (h *ClusterHandler) watch(ctx *gin.Context) {
	db := h.Collector.DB()
	err := h.Watch(
		ctx,
		db,
		&model.Cluster{},
		func(in libmodel.Model) (r interface{}) {
			pb := PathBuilder{DB: db}
			m := in.(*model.Cluster)
			cluster := &Cluster{}
			cluster.With(m)
			cluster.Link(h.Provider)
			cluster.Path = pb.Path(m)
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

// Filter result set.
func (h *ClusterHandler) filter(ctx *gin.Context, list *[]model.Cluster) (err error) {
	if len(*list) < 2 {
		return
	}
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) == 0 {
		return
	}
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	kept := []model.Cluster{}
	for _, m := range *list {
		path := pb.Path(&m)
		if path == name || strings.HasSuffix(path, "/"+name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// REST resource.
type Cluster struct {
	Resource
	ClusterUUID   string `json:"clusterUuid"`
	Version       string `json:"version"`
	BuildVersion  string `json:"buildVersion"`
	Timezone      string `json:"timezone"`
	ClusterArch   string `json:"clusterArch"`
	OperationMode string `json:"operationMode"`
	ExternalIP    string `json:"externalIp"`
	NumNodes      int    `json:"numNodes"`
	VMCount       int64  `json:"vmCount"`
	TotalCapacity int64  `json:"totalCapacity"`
	UsedCapacity  int64  `json:"usedCapacity"`
}

// Build the resource using the model.
func (r *Cluster) With(m *model.Cluster) {
	r.Resource.With(&m.Base)
	r.ClusterUUID = m.ClusterUUID
	r.Version = m.Version
	r.BuildVersion = m.BuildVersion
	r.Timezone = m.Timezone
	r.ClusterArch = m.ClusterArch
	r.OperationMode = m.OperationMode
	r.ExternalIP = m.ExternalIP
	r.NumNodes = m.NumNodes
	r.VMCount = m.VMCount
	r.TotalCapacity = m.TotalCapacity
	r.UsedCapacity = m.UsedCapacity
}

// Build self link (URI).
func (r *Cluster) Link(p *api.Provider) {
	r.SelfLink = base.Link(
		ClusterRoot,
		base.Params{
			base.ProviderParam: string(p.UID),
			ClusterParam:       r.ID,
		})
}

// As content.
func (r *Cluster) Content(detail int) interface{} {
	return r
}
