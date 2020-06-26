package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"net/http"
)

const (
	ClustersRoot     = Root + "/clusters"
	ClusterRoot      = ClustersRoot + "/:cluster"
	ClusterVmsRoot   = ClusterRoot + "/vms"
	ClusterHostsRoot = ClusterRoot + "/hosts"
)

//
// Cluster handler.
type ClusterHandler struct {
	Base
	cluster *model.Cluster
}

//
// Add routes to the `gin` router.
func (h *ClusterHandler) AddRoutes(e *gin.Engine) {
	e.GET(ClustersRoot, h.List)
	e.GET(ClustersRoot+"/", h.List)
	e.GET(ClusterRoot, h.Get)
	e.GET(ClusterVmsRoot, h.ListVM)
	e.GET(ClusterHostsRoot, h.ListHost)
}

//
// Prepare to handle the request.
func (h *ClusterHandler) Prepare(ctx *gin.Context) int {
	status := h.Base.Prepare(ctx)
	if status != http.StatusOK {
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
	content := []*Cluster{}
	for _, m := range list {
		r := &Cluster{}
		r.With(&m, false)
		content = append(content, r)
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
	r.With(h.cluster, true)

	ctx.JSON(http.StatusOK, r)
}

//
// List VMs
func (h ClusterHandler) ListVM(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	content := []VM{}
	tr := model.ClusterTraversal{
		Root: h.cluster,
		DB:   db,
	}
	traversed, err := tr.VmList()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, vm := range traversed {
		r := VM{}
		r.With(vm, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// List Hosts.
func (h ClusterHandler) ListHost(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	content := []Host{}
	tr := model.ClusterTraversal{
		Root: h.cluster,
		DB:   db,
	}
	traversed, err := tr.HostList()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, host := range traversed {
		r := Host{}
		r.With(host, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// REST Resource.
type Cluster struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Object model.Object `json:"object,omitempty"`
}

//
// Build the resource using the model.
func (r *Cluster) With(m *model.Cluster, detail bool) {
	r.ID = m.ID
	r.Name = m.Name
	if detail {
		r.Object = m.DecodeObject()
	}
}
