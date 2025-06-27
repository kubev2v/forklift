package ovirt

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
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
// A GET onn the collection that includes the `X-Watch`
// header will negotiate an upgrade of the connection
// to a websocket and push watch events.
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
// Filter by path for `name` query.
func (h *ClusterHandler) filter(ctx *gin.Context, list *[]model.Cluster) (err error) {
	if len(*list) < 2 {
		return
	}
	q := ctx.Request.URL.Query()
	name := q.Get(NameParam)
	if len(name) == 0 {
		return
	}
	if len(strings.Split(name, "/")) < 2 {
		return
	}
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	kept := []model.Cluster{}
	for _, m := range *list {
		path := pb.Path(&m)
		if h.PathMatchRoot(path, name) {
			kept = append(kept, m)
		}
	}

	*list = kept

	return
}

// REST Resource.
type Cluster struct {
	Resource
	DataCenter    string  `json:"dataCenter"`
	HaReservation bool    `json:"haReservation"`
	KsmEnabled    bool    `json:"ksmEnabled"`
	BiosType      string  `json:"biosType"`
	CPU           CPU     `json:"cpu"`
	Version       Version `json:"version"`
}

type CPU = model.CPU
type Version = model.Version

// Build the resource using the model.
func (r *Cluster) With(m *model.Cluster) {
	r.Resource.With(&m.Base)
	r.DataCenter = m.DataCenter
	r.HaReservation = m.HaReservation
	r.KsmEnabled = m.KsmEnabled
	r.BiosType = m.BiosType
	r.CPU = m.CPU
	r.Version = m.Version
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
	if detail == 0 {
		return r.Resource
	}

	return r
}
