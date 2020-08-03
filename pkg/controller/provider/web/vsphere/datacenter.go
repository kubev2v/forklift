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
	DatacentersRoot = Root + "/datacenters"
	DatacenterRoot  = DatacentersRoot + "/:datacenter"
	DcVmsRoot       = DatacenterRoot + "/vms"
	DcClustersRoot  = DatacenterRoot + "/clusters"
	DcHostsRoot     = DatacenterRoot + "/hosts"
	DcNetsRoot      = DatacenterRoot + "/networks"
	DcDssRoot       = DatacenterRoot + "/datastores"
	DcHostTree      = DatacenterRoot + "/tree/host"
	DcVmTree        = DatacenterRoot + "/tree/vm"
)

//
// Datacenter handler.
type DatacenterHandler struct {
	base.Handler
	// Selected Datacenter.
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
	e.GET(DcHostTree, h.HostTree)
	e.GET(DcVmTree, h.VmTree)
}

//
// Prepare to handle the request.
func (h *DatacenterHandler) Prepare(ctx *gin.Context) int {
	status := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
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
	content := []interface{}{}
	for _, m := range list {
		r := &Datacenter{}
		r.With(&m)
		obj := r.Object(h.Detail)
		content = append(content, obj)
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
	r.With(h.datacenter)

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
	tr := Tree{
		Root:    h.datacenter,
		Leaf:    model.VmKind,
		DB:      db,
		Flatten: true,
		Detail: map[string]bool{
			model.VmKind: h.Detail,
		},
	}
	content := []interface{}{}
	tree, err := tr.Build()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, node := range tree.Children {
		content = append(content, node.Object)
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
	tr := Tree{
		Root:    h.datacenter,
		Leaf:    model.ClusterKind,
		DB:      db,
		Flatten: true,
		Detail: map[string]bool{
			model.ClusterKind: h.Detail,
		},
	}
	content := []interface{}{}
	tree, err := tr.Build()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, node := range tree.Children {
		content = append(content, node.Object)
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
	tr := Tree{
		Root:    h.datacenter,
		Leaf:    model.HostKind,
		DB:      db,
		Flatten: true,
		Detail: map[string]bool{
			model.HostKind: h.Detail,
		},
	}
	content := []interface{}{}
	tree, err := tr.Build()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, node := range tree.Children {
		content = append(content, node.Object)
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
	tr := Tree{
		Root:    h.datacenter,
		Leaf:    model.NetKind,
		DB:      db,
		Flatten: true,
		Detail: map[string]bool{
			model.NetKind: h.Detail,
		},
	}
	content := []interface{}{}
	tree, err := tr.Build()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, node := range tree.Children {
		content = append(content, node.Object)
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
	tr := Tree{
		Root:    h.datacenter,
		Leaf:    model.DsKind,
		DB:      db,
		Flatten: true,
		Detail: map[string]bool{
			model.DsKind: h.Detail,
		},
	}
	content := []interface{}{}
	tree, err := tr.Build()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}
	for _, node := range tree.Children {
		content = append(content, node.Object)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// VM Tree.
func (h DatacenterHandler) VmTree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	tr := Tree{
		Root: h.datacenter,
		Leaf: model.VmKind,
		DB:   db,
		Detail: map[string]bool{
			model.VmKind: h.Detail,
		},
	}
	content, err := tr.Build()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Host Tree.
func (h DatacenterHandler) HostTree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	db := h.Reconciler.DB()
	tr := Tree{
		Root: h.datacenter,
		Leaf: model.HostKind,
		DB:   db,
		Detail: map[string]bool{
			model.HostKind: h.Detail,
		},
	}
	content, err := tr.Build()
	if err != nil {
		Log.Trace(err)
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

//
// REST Resource.
type Datacenter struct {
	base.Resource
}

//
// Build the resource using the model.
func (r *Datacenter) With(m *model.Datacenter) {
	r.Resource.With(&m.Base)
}

//
// Render.
func (r *Datacenter) Object(detail bool) interface{} {
	if !detail {
		return r.Resource
	}

	return r
}
