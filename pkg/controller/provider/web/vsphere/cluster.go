package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	model "github.com/konveyor/virt-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/virt-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	ClustersRoot = Root + "/clusters"
	ClusterRoot  = ClustersRoot + "/:cluster"
)

//
// Cluster handler.
type ClusterHandler struct {
	base.Handler
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
// Prepare to handle the request.
func (h *ClusterHandler) Prepare(ctx *gin.Context) int {
	status := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return status
	}
	id := ctx.Param("cluster")
	if id != "" {
		m := &model.Cluster{
			Base: model.Base{
				ID: id,
			},
		}
		db := h.Reconciler.DB()
		err := db.Get(m)
		if errors.Is(err, model.NotFound) {
			return http.StatusNotFound
		}
		if err != nil {
			Log.Trace(err)
			return http.StatusInternalServerError
		}

		h.cluster = m
	}

	return http.StatusOK
}

//
// List resources in a REST collection.
func (h ClusterHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	selector := &model.Cluster{}
	options := libmodel.ListOptions{
		Page: &h.Page,
	}
	list := []model.Cluster{}
	err := db.List(selector, options, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []interface{}{}
	for _, m := range list {
		r := &Cluster{}
		r.With(&m)
		obj := r.Object(h.Detail)
		content = append(content, obj)
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
	r := &Cluster{}
	r.With(h.cluster)

	ctx.JSON(http.StatusOK, r)
}

//
// REST Resource.
type Cluster struct {
	base.Resource
	Networks    model.RefList `json:"networks"`
	Datastores  model.RefList `json:"datastores"`
	DasEnabled  model.Bool    `json:"dasEnabled"`
	DasVms      model.RefList `json:"DasVms"`
	DrsEnabled  model.Bool    `json:"drsEnabled"`
	DrsBehavior string        `json:"drsBehavior"`
	DrsVms      model.RefList `json:"drsVms"`
}

//
// Build the resource using the model.
func (r *Cluster) With(m *model.Cluster) {
	r.Resource.With(&m.Base)
	r.DasEnabled = *model.BoolPtr(false).With(m.DasEnabled)
	r.DrsEnabled = *model.BoolPtr(false).With(m.DrsEnabled)
	r.DrsBehavior = m.DrsBehavior
	r.Networks = *model.RefListPtr().With(m.Networks)
	r.Datastores = *model.RefListPtr().With(m.Datastores)
	r.DasVms = *model.RefListPtr().With(m.DasVms)
	r.DrsVms = *model.RefListPtr().With(m.DasVms)
}

//
// Render.
func (r *Cluster) Object(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
