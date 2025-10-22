package vsphere

import (
	"net/http"

	"github.com/gin-gonic/gin"
	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/vsphere"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Routes.
const (
	TreeRoot     = ProviderRoot + "/tree"
	TreeHostRoot = TreeRoot + "/host"
	TreeVmRoot   = TreeRoot + "/vm"
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode

// Tree handler.
type TreeHandler struct {
	Handler
	// Datacenters list.
	datacenters []model.Datacenter
}

// Add routes to the `gin` router.
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	e.GET(TreeHostRoot, h.HostTree)
	e.GET(TreeVmRoot, h.VmTree)
}

// Prepare to handle the request.
func (h *TreeHandler) Prepare(ctx *gin.Context) int {
	status, err := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		base.SetForkliftError(ctx, err)
		return status
	}
	db := h.Collector.DB()
	err = db.List(
		&h.datacenters,
		model.ListOptions{
			Detail: model.MaxDetail,
		})
	if err != nil {
		log.Trace(
			err,
			"url",
			ctx.Request.URL)
		return http.StatusInternalServerError
	}

	return http.StatusOK
}

// List not supported.
func (h TreeHandler) List(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

// Get not supported.
func (h TreeHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

// VM Tree.
func (h TreeHandler) VmTree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusBadRequest)
		return
	}
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	content := TreeNode{}
	for _, dc := range h.datacenters {
		ref := dc.Vms
		folder := &model.Folder{
			Base: model.Base{
				ID: ref.ID,
			},
		}
		err := db.Get(folder)
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		tr := Tree{
			NodeBuilder: &NodeBuilder{
				provider:    h.Provider,
				pathBuilder: pb,
				detail: map[string]int{
					model.VmKind: h.Detail,
				},
			},
		}
		branch, err := tr.Build(
			folder,
			&VMNavigator{
				detail: h.Detail,
				db:     db,
			})
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r := Datacenter{}
		r.With(&dc)
		r.Link(h.Provider)
		r.Path = pb.Path(&dc)
		branch.Kind = model.DatacenterKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

// Cluster & Host Tree.
func (h TreeHandler) HostTree(ctx *gin.Context) {
	status := h.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return
	}
	if h.WatchRequest {
		ctx.Status(http.StatusBadRequest)
		return
	}
	db := h.Collector.DB()
	pb := PathBuilder{DB: db}
	content := TreeNode{}
	for _, dc := range h.datacenters {
		ref := dc.Clusters
		folder := &model.Folder{
			Base: model.Base{
				ID: ref.ID,
			},
		}
		err := db.Get(folder)
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		tr := Tree{
			NodeBuilder: &NodeBuilder{
				provider:    h.Provider,
				pathBuilder: pb,
				detail: map[string]int{
					model.VmKind: h.Detail,
				},
			},
		}
		branch, err := tr.Build(
			folder,
			&HostNavigator{
				detail: h.Detail,
				db:     db,
			})
		if err != nil {
			log.Trace(
				err,
				"url",
				ctx.Request.URL)
			ctx.Status(http.StatusInternalServerError)
			return
		}
		r := Datacenter{}
		r.With(&dc)
		r.Link(h.Provider)
		r.Path = pb.Path(&dc)
		branch.Kind = model.DatacenterKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

// Host (tree) navigator.
type HostNavigator struct {
	// DB.
	db libmodel.DB
	// VM detail.
	detail int
}

// Next (children) on the branch.
func (n *HostNavigator) Next(p libmodel.Model) (r []libmodel.Model, err error) {
	switch p := p.(type) {
	case *model.Datacenter:
		m := &model.Folder{
			Base: model.Base{
				ID: p.Clusters.ID,
			},
		}
		err = n.db.Get(m)
		if err == nil {
			r = []libmodel.Model{m}
		}
	case *model.Folder:
		folder := []model.Folder{}
		err = n.db.List(
			&folder,
			model.ListOptions{
				Predicate: libmodel.Eq("folder", p.Pk()),
			})
		if err == nil {
			for i := range folder {
				m := &folder[i]
				r = append(r, m)
			}
		} else {
			return
		}
		cluster := []model.Cluster{}
		err = n.db.List(
			&cluster,
			model.ListOptions{
				Predicate: libmodel.Eq("folder", p.Pk()),
			})
		if err == nil {
			for i := range cluster {
				m := &cluster[i]
				r = append(r, m)
			}
		} else {
			return
		}
	case *model.Cluster:
		list := []model.Host{}
		err = n.db.List(
			&list,
			model.ListOptions{
				Predicate: libmodel.Eq("cluster", p.Pk()),
			})
		if err == nil {
			for i := range list {
				m := &list[i]
				r = append(r, m)
			}
		} else {
			return
		}
	case *model.Host:
		detail := 0
		if n.detail > 0 {
			detail = model.MaxDetail
		}
		list := []model.VM{}
		err = n.db.List(
			&list,
			model.ListOptions{
				Predicate: libmodel.Eq("host", p.Pk()),
				Detail:    detail,
			})
		if err == nil {
			for i := range list {
				m := &list[i]
				r = append(r, m)
			}
		} else {
			return
		}
	}

	return
}

// VM (tree) navigator.
type VMNavigator struct {
	// DB.
	db libmodel.DB
	// VM detail.
	detail int
}

// Next (children) on the branch.
func (n *VMNavigator) Next(p libmodel.Model) (r []libmodel.Model, err error) {
	switch p := p.(type) {
	case *model.Datacenter:
		m := &model.Folder{
			Base: model.Base{ID: p.Clusters.ID},
		}
		err = n.db.Get(m)
		if err == nil {
			r = []libmodel.Model{m}
		}
	case *model.Folder:
		// Folder.
		folder := []model.Folder{}
		err = n.db.List(
			&folder,
			model.ListOptions{
				Predicate: libmodel.Eq("folder", p.Pk()),
			})
		if err == nil {
			for i := range folder {
				m := &folder[i]
				r = append(r, m)
			}
		} else {
			return
		}
		// VM
		detail := 0
		if n.detail > 0 {
			detail = model.MaxDetail
		}
		vm := []model.VM{}
		err = n.db.List(
			&vm,
			model.ListOptions{
				Predicate: libmodel.Eq("folder", p.Pk()),
				Detail:    detail,
			})
		if err == nil {
			for i := range vm {
				m := &vm[i]
				r = append(r, m)
			}
		} else {
			return
		}
	}

	return
}

// Tree node builder.
type NodeBuilder struct {
	// Provider.
	provider *api.Provider
	// Resource details by kind.
	detail map[string]int
	// Path builder.
	pathBuilder PathBuilder
}

// Build a node for the model.
func (r *NodeBuilder) Node(parent *TreeNode, m libmodel.Model) *TreeNode {
	kind := libref.ToKind(m)
	node := &TreeNode{}
	switch kind {
	case model.FolderKind:
		resource := &Folder{}
		resource.With(m.(*model.Folder))
		resource.Link(r.provider)
		resource.Path = r.pathBuilder.Path(m.(*model.Folder))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DatacenterKind:
		resource := &Datacenter{}
		resource.With(m.(*model.Datacenter))
		resource.Link(r.provider)
		resource.Path = r.pathBuilder.Path(m.(*model.Datacenter))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ClusterKind:
		resource := &Cluster{}
		resource.With(m.(*model.Cluster))
		resource.Link(r.provider)
		resource.Path = r.pathBuilder.Path(m.(*model.Cluster))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.HostKind:
		resource := &Host{}
		resource.With(m.(*model.Host))
		resource.Link(r.provider)
		resource.Path = r.pathBuilder.Path(m.(*model.Host))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VmKind:
		resource := &VM{}
		resource.With(m.(*model.VM))
		resource.Link(r.provider)
		resource.Path = r.pathBuilder.Path(m.(*model.VM))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetKind:
		resource := &Network{}
		resource.With(m.(*model.Network))
		resource.Link(r.provider)
		resource.Path = r.pathBuilder.Path(m.(*model.Network))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DsKind:
		resource := &Datastore{}
		resource.With(m.(*model.Datastore))
		resource.Link(r.provider)
		resource.Path = r.pathBuilder.Path(m.(*model.Datastore))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	}

	return node
}

// Build with detail.
func (r *NodeBuilder) withDetail(kind string) int {
	if b, found := r.detail[kind]; found {
		return b
	}

	return 0
}
