package ovirt

import (
	"net/http"

	"github.com/gin-gonic/gin"
	model "github.com/kubev2v/forklift/pkg/controller/provider/model/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/base"
	libmodel "github.com/kubev2v/forklift/pkg/lib/inventory/model"
	libref "github.com/kubev2v/forklift/pkg/lib/ref"
)

// Routes.
const (
	TreeRoot        = ProviderRoot + "/tree"
	TreeClusterRoot = TreeRoot + "/cluster"
)

// Types.
type Tree = base.Tree
type TreeNode = base.TreeNode

// Tree handler.
type TreeHandler struct {
	Handler
	// DataCenters list.
	datacenters []model.DataCenter
}

// Add routes to the `gin` router.
func (h *TreeHandler) AddRoutes(e *gin.Engine) {
	e.GET(TreeClusterRoot, h.Tree)
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

// Tree.
func (h TreeHandler) Tree(ctx *gin.Context) {
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
		tr := Tree{
			NodeBuilder: &NodeBuilder{
				handler:     h.Handler,
				pathBuilder: pb,
				detail: map[string]int{
					model.VmKind: h.Detail,
				},
			},
		}
		branch, err := tr.Build(
			&dc,
			&BranchNavigator{
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
		r := DataCenter{}
		r.With(&dc)
		r.Link(h.Provider)
		r.Path = pb.Path(&dc)
		branch.Kind = model.DataCenterKind
		branch.Object = r
		content.Children = append(content.Children, branch)
	}

	ctx.JSON(http.StatusOK, content)
}

// Tree (branch) navigator.
type BranchNavigator struct {
	db     libmodel.DB
	detail int
}

// Next (children) on the branch.
func (n *BranchNavigator) Next(p libmodel.Model) (r []model.Model, err error) {
	switch p := p.(type) {
	case *model.DataCenter:
		list, nErr := n.listCluster(p)
		if nErr == nil {
			for i := range list {
				m := &list[i]
				r = append(r, m)
			}
		} else {
			err = nErr
		}
	case *model.Cluster:
		hostList, nErr := n.listHost(p)
		if nErr == nil {
			for i := range hostList {
				m := &hostList[i]
				r = append(r, m)
			}
		} else {
			err = nErr
			return
		}
		vmList, nErr := n.listVM(p)
		if nErr == nil {
			for i := range vmList {
				m := &vmList[i]
				r = append(r, m)
			}
		} else {
			err = nErr
		}
	}

	return
}

func (n *BranchNavigator) listCluster(p *model.DataCenter) (list []model.Cluster, err error) {
	list = []model.Cluster{}
	err = n.db.List(
		&list,
		model.ListOptions{
			Predicate: libmodel.Eq("DataCenter", p.ID),
		})
	return
}

func (n *BranchNavigator) listHost(p *model.Cluster) (list []model.Host, err error) {
	list = []model.Host{}
	err = n.db.List(
		&list,
		model.ListOptions{
			Predicate: libmodel.Eq("Cluster", p.ID),
		})
	return
}

func (n *BranchNavigator) listVM(p *model.Cluster) (list []model.VM, err error) {
	detail := 0
	if n.detail > 0 {
		detail = model.MaxDetail
	}
	list = []model.VM{}
	err = n.db.List(
		&list,
		model.ListOptions{
			Predicate: libmodel.Eq("Cluster", p.ID),
			Detail:    detail,
		})
	return
}

// Tree node builder.
type NodeBuilder struct {
	// Handler.
	handler Handler
	// Resource details by kind.
	detail map[string]int
	// Path builder.
	pathBuilder PathBuilder
}

// Build a node for the model.
func (r *NodeBuilder) Node(parent *TreeNode, m model.Model) *TreeNode {
	provider := r.handler.Provider
	kind := libref.ToKind(m)
	node := &TreeNode{}
	switch kind {
	case model.DataCenterKind:
		resource := &DataCenter{}
		resource.With(m.(*model.DataCenter))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ClusterKind:
		resource := &Cluster{}
		resource.With(m.(*model.Cluster))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.HostKind:
		resource := &Host{}
		resource.With(m.(*model.Host))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.VmKind:
		resource := &VM{}
		resource.With(m.(*model.VM))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.NetKind:
		resource := &Network{}
		resource.With(m.(*model.Network))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.StorageKind:
		resource := &StorageDomain{}
		resource.With(m.(*model.StorageDomain))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
		object := resource.Content(r.withDetail(kind))
		node = &TreeNode{
			Parent: parent,
			Kind:   kind,
			Object: object,
		}
	case model.ServerCPUKind:
		resource := &ServerCpu{}
		resource.With(m.(*model.ServerCpu))
		resource.Link(provider)
		resource.Path = r.pathBuilder.Path(m)
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
