package vsphere

import (
	"github.com/gin-gonic/gin"
	libmodel "github.com/konveyor/controller/pkg/inventory/model"
	libref "github.com/konveyor/controller/pkg/ref"
	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	model "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/web/base"
	"net/http"
)

//
// Routes.
const (
	TreeRoot     = ProviderRoot + "/tree"
	TreeHostRoot = TreeRoot + "/host"
	TreeVmRoot   = TreeRoot + "/vm"
)

//
// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode

//
// Tree handler.
type TreeHandler struct {
	Handler
	// Datacenters list.
	datacenters []model.Datacenter
}

//
// Add routes to the `gin` router.
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	e.GET(TreeHostRoot, h.HostTree)
	e.GET(TreeVmRoot, h.VmTree)
}

//
// Prepare to handle the request.
func (h *TreeHandler) Prepare(ctx *gin.Context) int {
	status := h.Handler.Prepare(ctx)
	if status != http.StatusOK {
		ctx.Status(status)
		return status
	}
	db := h.Reconciler.DB()
	err := db.List(
		&h.datacenters,
		model.ListOptions{
			Detail: 1,
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

//
// List not supported.
func (h TreeHandler) List(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
// Get not supported.
func (h TreeHandler) Get(ctx *gin.Context) {
	ctx.Status(http.StatusMethodNotAllowed)
}

//
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
	db := h.Reconciler.DB()
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
				provider: h.Provider,
				detail: map[string]bool{
					model.VmKind: h.Detail,
				},
			},
		}
		branch, err := tr.Build(
			folder,
			&BranchNavigator{
				db: db,
				branch: func(m libmodel.Model) (refs []model.Ref) {
					switch m.(type) {
					case *model.Folder:
						refs = m.(*model.Folder).Children
					}
					return
				},
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
		r.SelfLink = DatacenterHandler{}.Link(h.Provider, &dc)
		branch.Kind = model.DatacenterKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

//
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
	db := h.Reconciler.DB()
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
				provider: h.Provider,
				detail: map[string]bool{
					model.ClusterKind: h.Detail,
					model.HostKind:    h.Detail,
					model.VmKind:      h.Detail,
				},
			},
		}
		branch, err := tr.Build(
			folder,
			&BranchNavigator{
				db: db,
				branch: func(m libmodel.Model) (refs []model.Ref) {
					switch m.(type) {
					case *model.Folder:
						refs = m.(*model.Folder).Children
					case *model.Cluster:
						refs = m.(*model.Cluster).Hosts
					case *model.Host:
						refs = m.(*model.Host).Vms
					}

					return
				},
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
		r.SelfLink = DatacenterHandler{}.Link(h.Provider, &dc)
		branch.Kind = model.DatacenterKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

//
// Tree navigator.
type BranchNavigator struct {
	// branch navigation.
	branch func(m libmodel.Model) (refs []model.Ref)
	db     libmodel.DB
}

//
// Get referenced model.
func (n *BranchNavigator) Next(p model.Model) (r []model.Model, err error) {
	for _, ref := range n.branch(p) {
		var m model.Model
		m, err = n.get(ref)
		if err == nil {
			r = append(r, m)
		} else {
			return
		}
	}
	return
}

//
// Get referenced model.
func (n *BranchNavigator) get(ref model.Ref) (r model.Model, err error) {
	base := model.Base{
		ID: ref.ID,
	}
	switch ref.Kind {
	case model.FolderKind:
		r = &model.Folder{Base: base}
	case model.DatacenterKind:
		r = &model.Datacenter{Base: base}
	case model.ClusterKind:
		r = &model.Cluster{Base: base}
	case model.HostKind:
		r = &model.Host{Base: base}
	case model.VmKind:
		r = &model.VM{Base: base}
	case model.NetKind:
		r = &model.Network{Base: base}
	case model.DsKind:
		r = &model.Datastore{Base: base}
	default:
		err = model.InvalidRefError{Ref: ref}
	}
	if r != nil {
		err = n.db.Get(r)
	}

	return
}

//
// Tree node builder.
type NodeBuilder struct {
	// Provider.
	provider *api.Provider
	// Resource details by kind.
	detail map[string]bool
}

//
// Build a node for the model.
func (r *NodeBuilder) Node(parent *TreeNode, m model.Model) *TreeNode {
	kind := libref.ToKind(m)
	node := &TreeNode{}
	switch kind {
	case model.FolderKind:
		resource := &Folder{}
		resource.With(m.(*model.Folder))
		resource.SelfLink =
			FolderHandler{}.Link(r.provider, m.(*model.Folder))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DatacenterKind:
		resource := &Datacenter{}
		resource.With(m.(*model.Datacenter))
		resource.SelfLink =
			DatacenterHandler{}.Link(r.provider, m.(*model.Datacenter))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ClusterKind:
		resource := &Cluster{}
		resource.With(m.(*model.Cluster))
		resource.SelfLink =
			ClusterHandler{}.Link(r.provider, m.(*model.Cluster))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.HostKind:
		resource := &Host{}
		resource.With(m.(*model.Host))
		resource.SelfLink =
			HostHandler{}.Link(r.provider, m.(*model.Host))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VmKind:
		resource := &VM{}
		resource.With(m.(*model.VM))
		resource.SelfLink =
			VMHandler{}.Link(r.provider, m.(*model.VM))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetKind:
		resource := &Network{}
		resource.With(m.(*model.Network))
		resource.SelfLink =
			NetworkHandler{}.Link(r.provider, m.(*model.Network))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.DsKind:
		resource := &Datastore{}
		resource.With(m.(*model.Datastore))
		resource.SelfLink =
			DatastoreHandler{}.Link(r.provider, m.(*model.Datastore))
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	}

	return node
}

//
// Build with detail.
func (r *NodeBuilder) withDetail(kind string) bool {
	if b, found := r.detail[kind]; found {
		return b
	}

	return false
}
