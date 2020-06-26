package web

import (
	"errors"
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	"github.com/konveyor/virt-controller/pkg/controller/provider/model"
	"net/http"
)

const (
	DatacentersRoot = Root + "/datacenters"
	DatacenterRoot  = DatacentersRoot + "/:datacenter"
	DcVmsRoot       = DatacenterRoot + "/vms"
	DcClustersRoot  = DatacenterRoot + "/clusters"
	DcHostsRoot     = DatacenterRoot + "/hosts"
	DcNetsRoot      = DatacenterRoot + "/networks"
	DcDssRoot       = DatacenterRoot + "/datastores"
)

//
// Datacenter handler.
type DatacenterHandler struct {
	Base
	datacenter *model.Datacenter
}

//
// Add routes to the `gin` router.
func (h *DatacenterHandler) AddRoutes(e *gin.Engine) {
	e.GET(DatacentersRoot, h.List)
	e.GET(DatacentersRoot+"/", h.List)
	e.GET(DatacenterRoot, h.Get)
	e.GET(DcVmsRoot, h.ListVM)
	e.GET(DcClustersRoot, h.ListCluster)
	e.GET(DcHostsRoot, h.ListHost)
	e.GET(DcNetsRoot, h.ListNetwork)
	e.GET(DcDssRoot, h.ListDatastore)
}

//
// Prepare to handle the request.
func (h *DatacenterHandler) Prepare(ctx *gin.Context) int {
	status := h.Base.Prepare(ctx)
	if status != http.StatusOK {
		return status
	}
	id := ctx.Param("datacenter")
	if id != "" {
		m := &model.Datacenter{
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

		h.datacenter = m
	}

	return http.StatusOK
}

//
// List resources in a REST collection.
func (h DatacenterHandler) List(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	selector := &model.Datacenter{}
	options := libmodel.ListOptions{
		Page: &h.Page,
	}
	list := []model.Datacenter{}
	err := db.List(selector, options, &list)
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	content := []*Datacenter{}
	for _, m := range list {
		r := &Datacenter{}
		r.With(&m, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Get a specific REST resource.
func (h DatacenterHandler) Get(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	r := &Datacenter{}
	r.With(h.datacenter, true)

	ctx.JSON(http.StatusOK, r)
}

//
// List VMs
func (h DatacenterHandler) ListVM(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	content := []VM{}
	tr := model.DatacenterTraversal{
		Root: h.datacenter,
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
// List Clusters.
func (h DatacenterHandler) ListCluster(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	content := []Cluster{}
	tr := model.DatacenterTraversal{
		Root: h.datacenter,
		DB:   db,
	}
	traversed, err := tr.ClusterList()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, cluster := range traversed {
		r := Cluster{}
		r.With(cluster, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// List Hosts.
func (h DatacenterHandler) ListHost(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	content := []Host{}
	tr := model.DatacenterTraversal{
		Root: h.datacenter,
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
// List Networks.
func (h DatacenterHandler) ListNetwork(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	content := []Network{}
	tr := model.DatacenterTraversal{
		Root: h.datacenter,
		DB:   db,
	}
	traversed, err := tr.NetList()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, net := range traversed {
		r := Network{}
		r.With(net, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// List Datastore.
func (h DatacenterHandler) ListDatastore(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	content := []Datastore{}
	tr := model.DatacenterTraversal{
		Root: h.datacenter,
		DB:   db,
	}
	traversed, err := tr.DsList()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, ds := range traversed {
		r := Datastore{}
		r.With(ds, false)
		content = append(content, r)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// REST Resource.
type Datacenter struct {
	ID     string       `json:"id"`
	Name   string       `json:"name"`
	Object model.Object `json:"object,omitempty"`
}

//
// Build the resource using the model.
func (r *Datacenter) With(m *model.Datacenter, detail bool) {
	r.ID = m.ID
	r.Name = m.Name
	if detail {
		r.Object = m.DecodeObject()
	}
}
